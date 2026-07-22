package analysis

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/operator-framework/operator-registry/alpha/declcfg"

	"github.com/midu16/opm-troubleshooting/internal/adhd"
	"github.com/midu16/opm-troubleshooting/internal/catalog"
	"github.com/midu16/opm-troubleshooting/internal/claudeapi"
	"github.com/midu16/opm-troubleshooting/internal/codeanalysis"
	"github.com/midu16/opm-troubleshooting/internal/datasource"
	"github.com/midu16/opm-troubleshooting/internal/gitdelta"
	"github.com/midu16/opm-troubleshooting/internal/healthcheck"
	"github.com/midu16/opm-troubleshooting/internal/imageinspect"
	"github.com/midu16/opm-troubleshooting/internal/learning"
	"github.com/midu16/opm-troubleshooting/internal/metadata"
	"github.com/midu16/opm-troubleshooting/internal/mustgather"
	"github.com/midu16/opm-troubleshooting/internal/noise"
	"github.com/midu16/opm-troubleshooting/internal/openshift"
	"github.com/midu16/opm-troubleshooting/internal/rag"
	rcamod "github.com/midu16/opm-troubleshooting/internal/rca"
	"github.com/midu16/opm-troubleshooting/internal/session"
	"github.com/midu16/opm-troubleshooting/internal/telco"
	"github.com/midu16/opm-troubleshooting/internal/workflow"
)

// AnalyzeMustGather orchestrates the complete must-gather fault analysis workflow.
func AnalyzeMustGather(ctx context.Context, cfg AnalysisConfig) (*AnalysisResult, error) {
	start := time.Now()

	result := &AnalysisResult{
		MustGatherPath: cfg.MustGatherPath,
		Environment:    cfg.Environment,
		FaultyReports:  make([]FaultReport, 0),
		RCADocuments:   make([]rcamod.Document, 0),
		Errors:         make([]error, 0),
	}

	if cfg.Environment == "" {
		cfg.Environment = noise.EnvProduction
	}
	if cfg.ClusterName == "" {
		cfg.ClusterName = session.DefaultClusterName(cfg.MustGatherPath)
	}
	result.ClusterName = cfg.ClusterName
	result.Environment = cfg.Environment

	// 1. Parse must-gather
	mgResult, err := mustgather.ParseMustGather(ctx, cfg.MustGatherPath)
	if err != nil {
		return nil, fmt.Errorf("parse must-gather: %w", err)
	}

	result.Operators = mgResult.Operators
	result.TotalOperators = len(mgResult.Operators)
	result.FaultyCount = mgResult.FaultyCount

	// Determine which operators to analyze
	targets := selectTargets(mgResult.Operators, cfg)
	if len(targets) == 0 {
		return result, nil
	}

	// 2. Render catalog if provided
	var catalogCfg *declcfg.DeclarativeConfig
	if cfg.CatalogRef != "" {
		catalogCfg, err = catalog.RenderCatalog(ctx, cfg.CatalogRef)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("render catalog: %w", err))
		}
	}

	// 3. Initialize Claude API client (optional)
	claudeClient, err := claudeapi.NewClient()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("claude API client: %w", err))
	}

	// 3b. Initialize RAG engine (optional)
	var ragEngine *rag.Engine
	if cfg.RAGEnabled {
		ragCfgPath := cfg.RAGConfigPath
		if ragCfgPath == "" {
			ragCfgPath = "rag-config.yaml"
		}
		ragCfg, ragErr := rag.LoadConfig(ragCfgPath)
		if ragErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: RAG config: %v\n", ragErr)
		} else {
			if cfg.RAGDataDir != "" {
				ragCfg.DataDir = cfg.RAGDataDir
			}
			ragEngine, err = rag.NewEngine(ragCfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: RAG engine: %v\n", err)
			} else {
				defer ragEngine.Close()
			}
		}
	}

	// 4a. Open metadata store for learning and correlation
	var metaStore *metadata.MetadataStore
	if cfg.MetadataDir != "" || cfg.EnableLearning || cfg.EnableRepoCorrelation {
		metaStore, err = metadata.Open(cfg.MetadataDir)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("metadata store: %w", err))
		} else {
			defer metaStore.Close()
			legacyDir := ""
			if cfg.StateDir != "" {
				legacyDir = cfg.StateDir
			}
			if legacyDir != "" {
				if migrated, mErr := metaStore.MigrateFromJSON(legacyDir); mErr == nil && migrated > 0 {
					fmt.Fprintf(os.Stderr, "Migrated %d legacy session(s) to metadata store\n", migrated)
				}
			}
		}
	}

	// 4b. Session store for cross-redeployment context
	var sessionStore *session.Store
	var sessionRecord *session.Record
	if cfg.StateDir != "" || cfg.ClusterName != "" {
		sessionStore, err = session.NewStore(cfg.StateDir)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("session store: %w", err))
		} else {
			pkg := cfg.PackageName
			if pkg == "" && len(targets) > 0 {
				pkg = targets[0].PackageName
			}
			sessionRecord, _ = sessionStore.Load(cfg.ClusterName, pkg)
			result.Session = sessionRecord
			cfg.Session = sessionRecord
		}
	}

	// 5. Analyze each target operator
	for _, op := range targets {
		report := analyzeSingleOperator(ctx, op, catalogCfg, cfg, claudeClient, metaStore, ragEngine)
		result.FaultyReports = append(result.FaultyReports, report)

		if cfg.GenerateRCA && report.RCADocument != nil {
			result.RCADocuments = append(result.RCADocuments, *report.RCADocument)
		}

		if len(report.Errors) == 0 {
			result.AnalyzedCount++
		}

		// Update session
		if sessionStore != nil && sessionRecord != nil {
			status := "healthy"
			realIssues := 0
			cosmetic := 0
			if report.NoiseReport != nil {
				realIssues = report.NoiseReport.RealIssues
				cosmetic = report.NoiseReport.CosmeticAlerts
			}
			if op.Faulty || realIssues > 0 {
				status = "failed"
			} else if report.HealthReport != nil && report.HealthReport.Warnings > 0 {
				status = "degraded"
			}

			entry := session.HistoryEntry{
				Summary:        summarizeReport(report),
				Status:         status,
				RealIssues:     realIssues,
				CosmeticAlerts: cosmetic,
				MustGatherPath: cfg.MustGatherPath,
			}
			sessionStore.RecordRun(sessionRecord, entry)

			if report.NoiseReport != nil {
				for _, f := range report.NoiseReport.Findings {
					if f.Classification == noise.ClassCosmetic {
						sessionStore.AddKnownCosmetic(sessionRecord, f.Dimension.Name+": "+f.NoiseReason)
					}
				}
			}
			_ = sessionStore.Save(sessionRecord)
		}
	}

	_ = start // reserved for future timing metrics
	return result, nil
}

func selectTargets(operators []mustgather.OperatorState, cfg AnalysisConfig) []mustgather.OperatorState {
	if cfg.TelcoSuite {
		return selectTelcoSuite(operators)
	}

	targets := make([]mustgather.OperatorState, 0)
	for _, op := range operators {
		if cfg.PackageName != "" && op.PackageName != cfg.PackageName {
			continue
		}
		// Analyze faulty operators, or all if health-check mode with explicit package
		if op.Faulty || (cfg.HealthCheck && cfg.PackageName != "") {
			targets = append(targets, op)
		}
	}

	// If specific package requested but not found as faulty, still analyze for health check
	if cfg.PackageName != "" && len(targets) == 0 {
		for _, op := range operators {
			if op.PackageName == cfg.PackageName {
				targets = append(targets, op)
				break
			}
		}
	}

	return targets
}

func selectTelcoSuite(operators []mustgather.OperatorState) []mustgather.OperatorState {
	packages := telco.PackageNames()
	targets := make([]mustgather.OperatorState, 0, len(packages)+1)

	for _, pkg := range packages {
		found := false
		for _, op := range operators {
			if op.PackageName == pkg {
				targets = append(targets, op)
				found = true
				break
			}
		}
		if !found {
			profile, ok := telco.ProfileByPackage(pkg)
			if ok {
				targets = append(targets, mustgather.OperatorState{
					PackageName: profile.PackageName,
					Namespace:   profile.DefaultNS,
					State:       "NotFound",
				})
			}
		}
	}

	// IDMS cluster config dimension (not an OLM subscription)
	targets = append(targets, mustgather.OperatorState{
		PackageName: "idms-mirror-check",
		Namespace:   "openshift-config",
		State:       "ClusterConfig",
	})

	return targets
}

func analyzeSingleOperator(
	ctx context.Context,
	op mustgather.OperatorState,
	catalogCfg *declcfg.DeclarativeConfig,
	cfg AnalysisConfig,
	claudeClient *claudeapi.Client,
	metaStore *metadata.MetadataStore,
	ragEngine *rag.Engine,
) FaultReport {
	report := FaultReport{
		Operator:        op,
		Errors:          make([]error, 0),
		RCAPatterns:     make([]rcamod.PatternMatch, 0),
		Recommendations: make([]rcamod.AnalysisRecommendation, 0),
	}

	// Resolve telco profile
	if profile, ok := telco.ProfileByPackage(op.PackageName); ok {
		report.TelcoProfile = &profile
	} else if op.PackageName == "idms-mirror-check" {
		idms := telco.IDMS()
		report.TelcoProfile = &idms
	}

	// Step 0: RCA pattern detection
	detector := rcamod.NewPatternDetector()
	patterns := detector.DetectPatterns(op.FailureReason)
	report.RCAPatterns = patterns
	for _, pattern := range patterns {
		if pattern.Confidence >= 0.5 {
			recommendations := rcamod.GetRecommendations(pattern.Pattern, op.FailureReason)
			report.Recommendations = append(report.Recommendations, recommendations...)
		}
	}

	// Step 0.5: Parse workloads for deep analysis
	ns := op.Namespace
	if report.TelcoProfile != nil && report.TelcoProfile.DefaultNS != "" && ns == "" {
		ns = report.TelcoProfile.DefaultNS
	}
	if ns != "" {
		workloads, _ := mustgather.ParseWorkloads(cfg.MustGatherPath, ns)
		report.Workloads = workloads
	}

	// Step 1: 20-dimension health check
	if cfg.HealthCheck {
		hcCfg := healthcheck.Config{
			MustGatherPath: cfg.MustGatherPath,
			Operator:       op,
		}
		if report.TelcoProfile != nil {
			hcCfg.Profile = report.TelcoProfile
		}
		hcReport, err := healthcheck.Run(ctx, hcCfg)
		if err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("health check: %w", err))
		} else {
			report.HealthReport = hcReport
			report.NoiseReport = noise.Filter(cfg.Environment, hcReport.Dimensions)
		}
	}

	// Step 2: Bundle metadata (skip for IDMS synthetic check)
	if op.PackageName != "idms-mirror-check" && catalogCfg != nil {
		installedBundleImage := findBundleImageForCSV(catalogCfg, op.PackageName, op.InstalledCSV)
		if installedBundleImage != "" {
			installedInfo, err := imageinspect.InspectBundle(ctx, installedBundleImage)
			if err != nil {
				report.Errors = append(report.Errors, fmt.Errorf("inspect installed bundle: %w", err))
			} else {
				report.InstalledBundle = installedInfo
			}
		}

		var targetBundleResult *workflow.Result
		var err error
		if cfg.TargetVersion != "" {
			targetBundleResult, err = workflow.InspectBundleFromConfig(ctx, catalogCfg, op.PackageName, op.Channel, cfg.TargetVersion)
		} else if op.Channel != "" {
			targetBundleResult, err = workflow.InspectChannelHeadFromConfig(ctx, catalogCfg, op.PackageName, op.Channel)
		}
		if err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("resolve target bundle: %w", err))
		} else if targetBundleResult != nil {
			report.TargetBundle = targetBundleResult.Info
		}
	}

	// Step 3: Git delta
	if report.InstalledBundle != nil && report.TargetBundle != nil &&
		report.InstalledBundle.Commit != "" && report.TargetBundle.Commit != "" &&
		report.TargetBundle.URL != "" {
		repoURL := extractRepoURL(report.TargetBundle.URL)
		if repoURL != "" {
			deltaReq := gitdelta.DeltaRequest{
				RepoURL:   repoURL,
				OldCommit: report.InstalledBundle.Commit,
				NewCommit: report.TargetBundle.Commit,
			}
			delta, err := gitdelta.CalculateDelta(ctx, deltaReq)
			if err != nil {
				report.Errors = append(report.Errors, fmt.Errorf("calculate git delta: %w", err))
			} else {
				report.CommitDelta = delta
			}
		}
	}

	// Step 4: Code-level source analysis
	codePatterns := buildCodePatterns(op, report.TelcoProfile)
	if cfg.SourceRepo != "" {
		caResult, err := codeanalysis.Analyze(ctx, codeanalysis.Config{
			RepoPath:       cfg.SourceRepo,
			SearchPatterns: codePatterns,
		})
		if err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("code analysis: %w", err))
		} else {
			report.CodeAnalysis = caResult
		}
	} else if report.TargetBundle != nil && report.TargetBundle.URL != "" {
		repoURL := extractRepoURL(report.TargetBundle.URL)
		if repoURL != "" && len(codePatterns) > 0 {
			commit := report.TargetBundle.Commit
			caResult, err := codeanalysis.CloneAndAnalyze(ctx, repoURL, commit, codePatterns)
			if err != nil {
				report.Errors = append(report.Errors, fmt.Errorf("clone code analysis: %w", err))
			} else {
				report.CodeAnalysis = caResult
			}
		}
	}

	// Step 5: Claude API analysis
	if claudeClient != nil && report.CommitDelta != nil {
		analysisReq := claudeapi.AnalysisRequest{
			OperatorName:     op.PackageName,
			FailureSymptoms:  op.FailureReason,
			InstalledVersion: getVersionString(report.InstalledBundle),
			TargetVersion:    getVersionString(report.TargetBundle),
			CommitDelta:      report.CommitDelta.DiffSummary,
			FilesChanged:     report.CommitDelta.FilesChanged,
		}
		analysis, err := claudeClient.AnalyzeFault(ctx, analysisReq)
		if err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("claude analysis: %w", err))
		} else {
			report.ClaudeAnalysis = analysis
		}
	}

	// Step 5.5: RAG knowledge base enrichment (deep analysis with fallback)
	if ragEngine != nil {
		deepInput := buildDeepTroubleshootInput(report, cfg)
		ragResult, ragErr := ragEngine.DeepTroubleshoot(ctx, deepInput)
		if ragErr != nil {
			fmt.Fprintf(os.Stderr, "  Warning: deep RAG: %v, falling back to shallow\n", ragErr)
			ragSymptoms := buildRAGSymptoms(report)
			ragResult, ragErr = ragEngine.Troubleshoot(ctx, op.PackageName, ragSymptoms, "")
		}
		if ragErr != nil {
			fmt.Fprintf(os.Stderr, "  Warning: RAG lookup: %v\n", ragErr)
		} else {
			report.RAGContext = ragResult
		}
	}

	// Step 6: Infrastructure health checks (via must-gather data source)
	mgSource := datasource.NewMustGatherSource(cfg.MustGatherPath)
	infraReport, err := healthcheck.RunInfra(healthcheck.InfraConfig{DataSource: mgSource})
	if err == nil {
		report.InfraReport = infraReport
	}

	// Step 7: ADHD multi-frame divergent analysis
	if cfg.ADHDEnabled && claudeClient != nil {
		symptoms := collectAnalysisSymptoms(report)
		if report.RAGContext != nil {
			for _, ki := range report.RAGContext.KnownIssues {
				symptoms = append(symptoms, fmt.Sprintf("[Known Issue %s] %s", ki.ID, ki.Summary))
			}
		}
		snapshot, snapErr := adhd.BuildClusterSnapshot(mgSource)
		if snapErr != nil {
			report.Errors = append(report.Errors, fmt.Errorf("cluster snapshot: %w", snapErr))
		}

		problem := fmt.Sprintf("Diagnose issues with operator %s in namespace %s", op.PackageName, op.Namespace)
		if op.FailureReason != "" {
			problem = fmt.Sprintf("Diagnose: %s (operator: %s)", op.FailureReason, op.PackageName)
		}

		opts := adhd.DiagnosisOptions{
			FrameCount:  cfg.ADHDFrames,
			TopK:        3,
			Depth:       cfg.ADHDDepth,
			Concurrency: 4,
		}
		if opts.FrameCount <= 0 {
			opts.FrameCount = 5
		}
		if opts.Depth == "" {
			opts.Depth = "standard"
		}

		engine := adhd.NewEngine(claudeClient)
		adhdResult, adhdErr := engine.Diagnose(ctx, problem, symptoms, snapshot, opts)
		if adhdErr != nil {
			report.Errors = append(report.Errors, fmt.Errorf("ADHD analysis: %w", adhdErr))
		} else {
			report.ADHDResult = adhdResult
		}
	}

	// Step 9: OpenShift repo correlation
	if cfg.EnableRepoCorrelation && op.FailureReason != "" {
		corrCfg := openshift.CorrelateConfig{
			CacheDir:    cfg.MetadataDir,
			InfraReport: report.InfraReport,
			SearchDays:  90,
		}

		if report.InstalledBundle != nil {
			corrCfg.BundleInfo = report.InstalledBundle
		} else if report.TargetBundle != nil {
			corrCfg.BundleInfo = report.TargetBundle
		}

		if report.CommitDelta != nil {
			corrCfg.CommitDelta = report.CommitDelta
		}

		correlationResult, err := openshift.Correlate(ctx, op.PackageName, op.FailureReason, corrCfg)
		if err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("repo correlation: %w", err))
		} else {
			report.RepoCorrelation = correlationResult
		}
	}

	// Step 10: Self-learning lookup
	if cfg.EnableLearning && metaStore != nil {
		symptomInput := learning.SymptomInput{
			Operator:     op,
			HealthReport: report.HealthReport,
			InfraReport:  report.InfraReport,
			NoiseReport:  report.NoiseReport,
			RCAPatterns:  report.RCAPatterns,
		}
		fp := learning.BuildFingerprint(symptomInput)
		similarIssues, err := learning.FindSimilarIssues(metaStore, fp)
		if err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("learning lookup: %w", err))
		} else {
			report.SimilarIssues = convertSimilarIssues(similarIssues)
		}
		insights, err := learning.BuildInsights(metaStore, op.PackageName, similarIssues)
		if err == nil {
			report.LearningInsights = convertInsights(insights)
		}
	}

	// Step 11: Record to metadata store
	if metaStore != nil {
		recordToMetadata(metaStore, cfg, op, report)
	}

	// Step 12: Generate RCA document
	if cfg.GenerateRCA {
		rcaInput := rcamod.ReportInput{
			ClusterName:      cfg.ClusterName,
			Environment:      cfg.Environment,
			MustGatherPath:   cfg.MustGatherPath,
			Operator:         op.PackageName,
			Namespace:        op.Namespace,
			OperatorState:    op,
			InstalledBundle:  report.InstalledBundle,
			TargetBundle:     report.TargetBundle,
			CommitDelta:      report.CommitDelta,
			ClaudeAnalysis:   report.ClaudeAnalysis,
			RCAPatterns:      report.RCAPatterns,
			Recommendations:  report.Recommendations,
			HealthReport:     report.HealthReport,
			InfraReport:      report.InfraReport,
			NoiseReport:      report.NoiseReport,
			CodeAnalysis:     report.CodeAnalysis,
			ADHDResult:       report.ADHDResult,
			Session:          cfg.Session,
			RepoCorrelation:  convertCorrelation(report.RepoCorrelation),
			RAGContext:       convertRAGContext(report.RAGContext),
			SimilarIssues:    report.SimilarIssues,
			LearningInsights: report.LearningInsights,
		}
		doc := rcamod.GenerateDocument(rcaInput)
		report.RCADocument = &doc
	}

	return report
}

func recordToMetadata(store *metadata.MetadataStore, cfg AnalysisConfig, op mustgather.OperatorState, report FaultReport) {
	sessID := session.DefaultClusterName(cfg.MustGatherPath) + ":" + op.PackageName
	_ = store.SaveSession(metadata.Session{
		ID:          sessID,
		ClusterName: cfg.ClusterName,
		Operator:    op.PackageName,
		Environment: string(cfg.Environment),
		SourceType:  "must-gather",
	})

	status := "healthy"
	if op.Faulty {
		status = "failed"
	} else if report.HealthReport != nil && report.HealthReport.Warnings > 0 {
		status = "degraded"
	}

	classification := ""
	if report.RepoCorrelation != nil {
		classification = report.RepoCorrelation.Classification.Type
	}

	run := metadata.Run{
		SessionID:      sessID,
		Status:         status,
		MustGatherPath: cfg.MustGatherPath,
		Classification: classification,
	}
	if report.NoiseReport != nil {
		run.RealIssues = report.NoiseReport.RealIssues
		run.CosmeticAlerts = report.NoiseReport.CosmeticAlerts
	}
	if report.HealthReport != nil {
		run.HealthPassed = report.HealthReport.Passed
		run.HealthFailed = report.HealthReport.Failed
	}
	if report.InfraReport != nil {
		run.InfraPassed = report.InfraReport.Passed
		run.InfraFailed = report.InfraReport.Failed
	}
	if report.ADHDResult != nil {
		run.ADHDBranches = len(report.ADHDResult.Branches)
		run.ADHDTraps = len(report.ADHDResult.Traps)
	}

	runID, err := store.RecordRun(run)
	if err != nil {
		return
	}

	symptomInput := learning.SymptomInput{
		Operator:     op,
		HealthReport: report.HealthReport,
		InfraReport:  report.InfraReport,
		NoiseReport:  report.NoiseReport,
		RCAPatterns:  report.RCAPatterns,
	}
	fp := learning.BuildFingerprint(symptomInput)
	fp.RunID = runID
	fp.Classification = classification
	_, _ = store.SaveFingerprint(fp)

	if report.ADHDResult != nil {
		var hyps []metadata.HypothesisRecord
		for _, branch := range report.ADHDResult.Branches {
			for _, h := range branch.Hypotheses {
				isTrap := false
				for _, t := range report.ADHDResult.Traps {
					if t.ID == h.ID {
						isTrap = true
						break
					}
				}
				hyps = append(hyps, metadata.HypothesisRecord{
					RunID:          runID,
					FrameID:        h.FrameID,
					HypothesisText: h.Text,
					ScoreTotal:     h.Score.Total,
					WasTrap:        isTrap,
					Operator:       op.PackageName,
					ClusterName:    cfg.ClusterName,
				})
			}
		}
		if len(hyps) > 0 {
			_ = store.RecordHypotheses(runID, hyps)
		}
	}

	for _, p := range report.RCAPatterns {
		_ = store.RecordPattern(string(p.Pattern), op.PackageName, cfg.ClusterName, p.Confidence)
	}
}

func buildCodePatterns(op mustgather.OperatorState, profile *telco.Profile) []string {
	extra := make([]string, 0)
	if profile != nil {
		extra = append(extra, profile.LogPatterns...)
	}
	if op.RootCause != nil && op.RootCause.RawFailureMessage != "" {
		extra = append(extra, op.RootCause.RawFailureMessage)
	}
	return codeanalysis.PatternsFromFailure(op.FailureReason, extra)
}

func summarizeReport(report FaultReport) string {
	if report.Operator.FailureReason != "" {
		return truncateStr(report.Operator.FailureReason, 120)
	}
	if report.HealthReport != nil {
		return fmt.Sprintf("%d/%d dimensions passed", report.HealthReport.Passed, report.HealthReport.TotalDimensions)
	}
	return "Analysis complete"
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// findBundleImageForCSV searches catalog for a bundle whose CSV name matches.
func findBundleImageForCSV(cfg *declcfg.DeclarativeConfig, packageName, csvName string) string {
	for _, bundle := range cfg.Bundles {
		if bundle.Package == packageName && bundle.Name == csvName {
			return bundle.Image
		}
	}
	return ""
}

func extractRepoURL(commitURL string) string {
	if idx := strings.Index(commitURL, "/commit/"); idx != -1 {
		return commitURL[:idx]
	}
	if idx := strings.Index(commitURL, "/-/commit/"); idx != -1 {
		return commitURL[:idx]
	}
	return commitURL
}

func getVersionString(info *imageinspect.BundleInfo) string {
	if info == nil {
		return "unknown"
	}
	return info.Version
}

func collectAnalysisSymptoms(report FaultReport) []string {
	symptoms := make([]string, 0)
	if report.Operator.FailureReason != "" {
		symptoms = append(symptoms, report.Operator.FailureReason)
	}
	if report.HealthReport != nil {
		for _, dim := range report.HealthReport.Dimensions {
			if dim.Status == healthcheck.StatusFail {
				symptoms = append(symptoms, fmt.Sprintf("[%s] %s", dim.Name, dim.Summary))
			}
		}
	}
	if report.InfraReport != nil {
		for _, dim := range report.InfraReport.Dimensions {
			if dim.Status == healthcheck.StatusFail {
				symptoms = append(symptoms, fmt.Sprintf("[infra:%s] %s", dim.Name, dim.Summary))
			}
		}
	}
	return symptoms
}

func buildRAGSymptoms(report FaultReport) []string {
	symptoms := make([]string, 0)
	if report.Operator.FailureReason != "" {
		symptoms = append(symptoms, report.Operator.FailureReason)
	}
	if report.Operator.CurrentCSV != "" {
		symptoms = append(symptoms, fmt.Sprintf("Current CSV: %s", report.Operator.CurrentCSV))
	}
	if report.HealthReport != nil {
		for _, dim := range report.HealthReport.Dimensions {
			if dim.Status == healthcheck.StatusFail {
				symptoms = append(symptoms, fmt.Sprintf("%s: %s", dim.Name, dim.Summary))
			}
		}
	}
	return symptoms
}

func buildDeepTroubleshootInput(report FaultReport, _ AnalysisConfig) rag.DeepTroubleshootInput {
	input := rag.DeepTroubleshootInput{
		Operator:          report.Operator.PackageName,
		Namespace:         report.Operator.Namespace,
		FailureReason:     report.Operator.FailureReason,
		CurrentCSV:        report.Operator.CurrentCSV,
		InstalledCSV:      report.Operator.InstalledCSV,
		Channel:           report.Operator.Channel,
		SubscriptionState: report.Operator.State,
	}

	if report.HealthReport != nil {
		for _, dim := range report.HealthReport.Dimensions {
			if dim.Status == healthcheck.StatusFail || dim.Status == healthcheck.StatusWarn {
				input.FailedDimensions = append(input.FailedDimensions, rag.DimensionSymptom{
					DimensionID: string(dim.ID),
					Name:        dim.Name,
					Category:    dim.Category,
					Status:      string(dim.Status),
					Summary:     dim.Summary,
					Evidence:    dim.Evidence,
				})
			}
		}
	}

	if report.InfraReport != nil {
		for _, dim := range report.InfraReport.Dimensions {
			if dim.Status == healthcheck.StatusFail || dim.Status == healthcheck.StatusWarn {
				input.InfraFailures = append(input.InfraFailures, rag.DimensionSymptom{
					DimensionID: string(dim.ID),
					Name:        dim.Name,
					Category:    dim.Category,
					Status:      string(dim.Status),
					Summary:     dim.Summary,
					Evidence:    dim.Evidence,
				})
			}
		}
	}

	if report.Workloads != nil {
		for _, pod := range report.Workloads.Pods {
			if pod.Phase != "Running" && pod.Phase != "Succeeded" || !pod.Ready || pod.WaitingReason != "" || pod.TerminatedReason != "" {
				input.UnhealthyPods = append(input.UnhealthyPods, rag.PodSymptom{
					Name:             pod.Name,
					Phase:            pod.Phase,
					WaitingReason:    pod.WaitingReason,
					WaitingMessage:   pod.WaitingMessage,
					TerminatedReason: pod.TerminatedReason,
					RestartCount:     pod.RestartCount,
				})
			}
		}
		for _, dep := range report.Workloads.Deployments {
			if !dep.Available || dep.UnavailableMsg != "" {
				input.UnavailableDeploy = append(input.UnavailableDeploy, rag.DeploymentSymptom{
					Name:           dep.Name,
					Replicas:       dep.Replicas,
					ReadyReplicas:  dep.ReadyReplicas,
					ProgressingMsg: dep.ProgressingMsg,
					UnavailableMsg: dep.UnavailableMsg,
				})
			}
		}
		for _, ev := range report.Workloads.Events {
			input.WarningEvents = append(input.WarningEvents, rag.EventSymptom{
				Object:  ev.Object,
				Reason:  ev.Reason,
				Message: ev.Message,
			})
		}
	}

	return input
}

func convertSimilarIssues(issues []learning.SimilarIssue) []rcamod.SimilarIssueData {
	if len(issues) == 0 {
		return nil
	}
	result := make([]rcamod.SimilarIssueData, len(issues))
	for i, si := range issues {
		result[i] = rcamod.SimilarIssueData{
			Operator:       si.Operator,
			Classification: si.Classification,
			Resolution:     si.Resolution,
			Similarity:     si.Similarity,
			HitCount:       si.HitCount,
		}
	}
	return result
}

func convertInsights(insights *learning.Insights) *rcamod.LearningInsightsData {
	if insights == nil {
		return nil
	}
	result := &rcamod.LearningInsightsData{}
	for _, fs := range insights.FrameStats {
		result.FrameStats = append(result.FrameStats, rcamod.FrameStatData{
			FrameID:   fs.FrameID,
			Total:     fs.Total,
			Confirmed: fs.Confirmed,
			TrapCount: fs.TrapCount,
		})
	}
	for _, ps := range insights.TopPatterns {
		result.TopPatterns = append(result.TopPatterns, rcamod.PatternStatData{
			Pattern:    ps.Pattern,
			Count:      ps.Count,
			Confidence: ps.Confidence,
		})
	}
	return result
}

func convertCorrelation(c *openshift.Correlation) *rcamod.RepoCorrelationData {
	if c == nil {
		return nil
	}
	result := &rcamod.RepoCorrelationData{
		Operator: c.Operator,
		RepoPath: c.RepoPath,
		RepoURL:  c.RepoURL,
		Classification: rcamod.ClassificationData{
			Type:       c.Classification.Type,
			Confidence: c.Classification.Confidence,
			Evidence:   c.Classification.Evidence,
		},
		Evidence:        c.Evidence,
		Recommendation:  c.Recommendation,
		BundleCommit:    c.BundleCommit,
		RepoSource:      c.RepoSource,
		RepoVerified:    c.RepoVerified,
		CommitPinned:    c.CommitPinned,
		ConfidenceGrade: c.ConfidenceGrade,
	}
	for _, issue := range c.GitHubIssues {
		result.GitHubIssues = append(result.GitHubIssues, rcamod.GitHubIssueData{
			Number:    issue.Number,
			Title:     issue.Title,
			State:     issue.State,
			URL:       issue.URL,
			UpdatedAt: issue.UpdatedAt.Format("2006-01-02"),
		})
	}
	for _, commit := range c.RecentCommits {
		result.RecentCommits = append(result.RecentCommits, rcamod.CommitData{
			Hash:    commit.Hash,
			Subject: commit.Subject,
			Author:  commit.Author,
		})
	}
	return result
}

func convertRAGContext(r *rag.TroubleshootResult) *rcamod.RAGContextData {
	if r == nil {
		return nil
	}
	result := &rcamod.RAGContextData{
		Summary:                r.Summary,
		Confidence:             r.Confidence,
		IssueClassification:    r.IssueClassification,
		ClassificationEvidence: r.ClassificationEvidence,
	}
	for _, ref := range r.DocumentationRefs {
		result.DocumentationRefs = append(result.DocumentationRefs, rcamod.RAGDocRef{
			Title:   ref.Title,
			Source:  ref.Source,
			Excerpt: ref.Excerpt,
			URL:     ref.URL,
		})
	}
	for _, ki := range r.KnownIssues {
		result.KnownIssues = append(result.KnownIssues, rcamod.RAGKnownIssue{
			ID:         ki.ID,
			Summary:    ki.Summary,
			Workaround: ki.Workaround,
			FixVersion: ki.FixVersion,
		})
	}
	for _, ca := range r.ConfigAdvice {
		result.ConfigAdvice = append(result.ConfigAdvice, rcamod.RAGConfigAdvice{
			Component: ca.Component,
			Reference: ca.Reference,
			Advice:    ca.Advice,
		})
	}
	for _, se := range r.SymptomAnalysis {
		rse := rcamod.RAGSymptomEvidence{
			Symptom:     se.Symptom,
			DimensionID: se.DimensionID,
			Relevance:   se.Relevance,
		}
		for _, d := range se.DocMatches {
			rse.DocMatches = append(rse.DocMatches, rcamod.RAGDocRef{
				Title: d.Title, Source: d.Source, Excerpt: d.Excerpt, URL: d.URL,
			})
		}
		for _, d := range se.CodeMatches {
			rse.CodeMatches = append(rse.CodeMatches, rcamod.RAGDocRef{
				Title: d.Title, Source: d.Source, Excerpt: d.Excerpt, URL: d.URL,
			})
		}
		for _, ca := range se.ConfigMatches {
			rse.ConfigMatches = append(rse.ConfigMatches, rcamod.RAGConfigAdvice{
				Component: ca.Component, Reference: ca.Reference, Advice: ca.Advice,
			})
		}
		for _, ki := range se.KnownIssues {
			rse.KnownIssues = append(rse.KnownIssues, rcamod.RAGKnownIssue{
				ID: ki.ID, Summary: ki.Summary, Workaround: ki.Workaround, FixVersion: ki.FixVersion,
			})
		}
		result.SymptomAnalysis = append(result.SymptomAnalysis, rse)
	}
	for _, rs := range r.RemediationSteps {
		result.RemediationSteps = append(result.RemediationSteps, rcamod.RAGRemediationStep{
			Step: rs.Step, Priority: rs.Priority, Action: rs.Action,
			Source: rs.Source, Confidence: rs.Confidence,
		})
	}
	for _, cp := range r.RelevantCodePaths {
		result.RelevantCodePaths = append(result.RelevantCodePaths, rcamod.RAGCodePathEvidence{
			Declaration: cp.Declaration, FilePath: cp.FilePath, Repo: cp.Repo,
			RepoURL: cp.RepoURL, Excerpt: cp.Excerpt, Relevance: cp.Relevance,
		})
	}
	return result
}
