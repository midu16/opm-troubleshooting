package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/midu16/opm-troubleshooting/internal/adhd"
	"github.com/midu16/opm-troubleshooting/internal/claudeapi"
	"github.com/midu16/opm-troubleshooting/internal/datasource"
	"github.com/midu16/opm-troubleshooting/internal/healthcheck"
	"github.com/midu16/opm-troubleshooting/internal/learning"
	"github.com/midu16/opm-troubleshooting/internal/livecluster"
	"github.com/midu16/opm-troubleshooting/internal/metadata"
	"github.com/midu16/opm-troubleshooting/internal/noise"
	"github.com/midu16/opm-troubleshooting/internal/openshift"
	"github.com/midu16/opm-troubleshooting/internal/rag"
	"github.com/midu16/opm-troubleshooting/internal/rca"
)

// diagnoseConfig holds configuration for the unified opm-diagnose command.
type diagnoseConfig struct {
	mustGatherPath string
	kubeconfigPath string
	kubeContext    string
	catalog        string
	packageName    string
	version        string
	jsonOut        bool
	timeout        time.Duration

	telcoSuite  bool
	healthCheck bool
	environment string
	sourceRepo  string
	generateRCA bool
	rcaFile     string
	clusterName string
	stateDir    string

	adhdEnabled bool
	frameCount  int
	depth       string // "quick", "standard", "deep"

	metadataDir    string
	enableLearning bool
	enableRepoCorr bool
	githubToken    string

	ragEnabled    bool
	ragConfigPath string
	ragDataDir    string
}

// RunDiagnose executes the unified diagnostic CLI supporting both kubeconfig and must-gather.
func RunDiagnose(args []string) error {
	cfg, err := parseDiagnoseArgs(args)
	if err != nil {
		if errors.Is(err, errHelp) {
			return nil
		}
		return &CLIError{Code: exitUsage, Err: err}
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	if cfg.kubeconfigPath != "" {
		return runLiveClusterAnalysis(ctx, cfg)
	}
	return runMustGatherDiagnosis(ctx, cfg)
}

func parseDiagnoseArgs(args []string) (*diagnoseConfig, error) {
	args = preprocessMustGatherArgs(args)

	fs := flag.NewFlagSet("opm-diagnose", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		printDiagnoseUsage(os.Stdout)
	}

	cfg := &diagnoseConfig{}

	// Data source flags
	fs.StringVar(&cfg.mustGatherPath, "must-gather", "", "Path or glob to must-gather directory")
	fs.StringVar(&cfg.mustGatherPath, "m", "", "Must-gather path (shorthand)")
	fs.StringVar(&cfg.kubeconfigPath, "kubeconfig", "", "Path to kubeconfig file (default: KUBECONFIG env, then ~/.kube/config)")
	fs.StringVar(&cfg.kubeContext, "context", "", "Kubernetes context to use (default: current-context)")

	// Bundle analysis flags
	fs.StringVar(&cfg.catalog, "catalog", "", "OLM catalog index image for bundle/code delta analysis")
	fs.StringVar(&cfg.catalog, "c", "", "Catalog index image (shorthand)")
	fs.StringVar(&cfg.packageName, "package", "", "Operator package name (default: all faulty, or full telco suite)")
	fs.StringVar(&cfg.packageName, "p", "", "Package name (shorthand)")
	fs.StringVar(&cfg.version, "version", "", "Target bundle version for code delta comparison")

	// Analysis options
	fs.BoolVar(&cfg.telcoSuite, "telco-suite", false, "Analyze full telco operator suite (27 operators + IDMS)")
	fs.BoolVar(&cfg.healthCheck, "health-check", true, "Run systematic health check dimensions")
	fs.StringVar(&cfg.environment, "environment", "production", "Noise filter: production, disconnected, lab, kvm")
	fs.StringVar(&cfg.sourceRepo, "source-repo", "", "Local operator source repo for code-level correlation")
	fs.BoolVar(&cfg.generateRCA, "rca", true, "Generate RCA markdown report")
	fs.StringVar(&cfg.rcaFile, "rca-file", "", "Write RCA markdown to file")
	fs.StringVar(&cfg.clusterName, "cluster-name", "", "Cluster identifier for session persistence")
	fs.StringVar(&cfg.stateDir, "state-dir", "", "Session state directory")

	// ADHD analysis flags
	fs.BoolVar(&cfg.adhdEnabled, "adhd", true, "Enable ADHD multi-frame divergent analysis (requires ANTHROPIC_API_KEY)")
	fs.IntVar(&cfg.frameCount, "frames", 5, "Number of ADHD divergence frames")
	fs.StringVar(&cfg.depth, "depth", "standard", "Analysis depth: quick (3 frames), standard (5), deep (8+)")

	// Metadata, learning, and repo correlation flags
	fs.StringVar(&cfg.metadataDir, "metadata-dir", "", "Metadata store directory (default: ~/.config/opm-troubleshooting)")
	fs.BoolVar(&cfg.enableLearning, "learning", true, "Enable self-learning from past sessions")
	fs.BoolVar(&cfg.enableRepoCorr, "repo-correlation", true, "Enable OpenShift repo correlation for issue classification")
	fs.StringVar(&cfg.githubToken, "github-token", "", "GitHub token for issue search (default: GH_TOKEN env var)")

	// RAG knowledge base flags
	fs.BoolVar(&cfg.ragEnabled, "rag", false, "Enable RAG knowledge base enrichment")
	fs.StringVar(&cfg.ragConfigPath, "rag-config", "", "RAG config file (default: rag-config.yaml)")
	fs.StringVar(&cfg.ragDataDir, "rag-data", "", "RAG data directory (default: rag-data)")

	// Output flags
	fs.BoolVar(&cfg.jsonOut, "json", false, "Output JSON")
	fs.DurationVar(&cfg.timeout, "timeout", defaultTimeout, "Overall timeout")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, errHelp
		}
		return nil, err
	}

	if cfg.timeout <= 0 {
		cfg.timeout = defaultTimeout
	}

	// Validate: need either must-gather or kubeconfig
	if cfg.mustGatherPath == "" && cfg.kubeconfigPath == "" {
		// Try KUBECONFIG env var
		if envKC := os.Getenv("KUBECONFIG"); envKC != "" {
			cfg.kubeconfigPath = envKC
		} else {
			// Try default kubeconfig location
			home, _ := os.UserHomeDir()
			defaultKC := filepath.Join(home, ".kube", "config")
			if _, err := os.Stat(defaultKC); err == nil {
				cfg.kubeconfigPath = defaultKC
			}
		}

		if cfg.kubeconfigPath == "" && cfg.mustGatherPath == "" {
			return nil, errors.New("either --must-gather or --kubeconfig is required")
		}
	}

	// Adjust frame count based on depth
	switch cfg.depth {
	case "quick":
		if cfg.frameCount == 5 { // only override if user didn't specify
			cfg.frameCount = 3
		}
	case "deep":
		if cfg.frameCount == 5 {
			cfg.frameCount = 8
		}
	case "standard":
		// keep defaults
	default:
		return nil, fmt.Errorf("invalid --depth: %s (must be quick, standard, or deep)", cfg.depth)
	}

	if cfg.githubToken == "" {
		if t := os.Getenv("GH_TOKEN"); t != "" {
			cfg.githubToken = t
		} else if t := os.Getenv("GITHUB_TOKEN"); t != "" {
			cfg.githubToken = t
		}
	}

	return cfg, nil
}

// runLiveClusterAnalysis connects to a live cluster via kubeconfig and runs analysis.
func runLiveClusterAnalysis(ctx context.Context, cfg *diagnoseConfig) error {
	fmt.Fprintf(os.Stderr, "Connecting to live cluster via kubeconfig: %s\n", cfg.kubeconfigPath)
	if cfg.kubeContext != "" {
		fmt.Fprintf(os.Stderr, "Using context: %s\n", cfg.kubeContext)
	}

	src, err := livecluster.NewLiveClusterSource(cfg.kubeconfigPath, cfg.kubeContext)
	if err != nil {
		return &CLIError{Code: exitOperation, Err: fmt.Errorf("connect to cluster: %w", err)}
	}

	return runDataSourceAnalysis(ctx, src, cfg)
}

// runMustGatherDiagnosis runs analysis from must-gather with the unified config.
func runMustGatherDiagnosis(ctx context.Context, cfg *diagnoseConfig) error {
	mgCfg := &config{
		mustGatherPath: cfg.mustGatherPath,
		catalog:        cfg.catalog,
		packageName:    cfg.packageName,
		version:        cfg.version,
		jsonOut:        cfg.jsonOut,
		timeout:        cfg.timeout,
		telcoSuite:     cfg.telcoSuite,
		healthCheck:    cfg.healthCheck,
		environment:    cfg.environment,
		sourceRepo:     cfg.sourceRepo,
		generateRCA:    cfg.generateRCA,
		rcaFile:        cfg.rcaFile,
		clusterName:    cfg.clusterName,
		stateDir:       cfg.stateDir,
		adhdEnabled:    cfg.adhdEnabled,
		adhdFrames:     cfg.frameCount,
		adhdDepth:      cfg.depth,
		metadataDir:    cfg.metadataDir,
		enableLearning: cfg.enableLearning,
		enableRepoCorr: cfg.enableRepoCorr,
		githubToken:    cfg.githubToken,
		ragEnabled:     cfg.ragEnabled,
		ragConfigPath:  cfg.ragConfigPath,
		ragDataDir:     cfg.ragDataDir,
	}
	return runMustGatherAnalysis(ctx, mgCfg)
}

// runDataSourceAnalysis runs the full analysis pipeline on a ClusterDataSource.
func runDataSourceAnalysis(ctx context.Context, src datasource.ClusterDataSource, cfg *diagnoseConfig) error {
	fmt.Fprintf(os.Stderr, "Collecting cluster state from %s...\n", src.SourceType())

	env := noise.ParseEnvironment(cfg.environment)

	// 1. Infrastructure health checks
	var infraReport *healthcheck.Report
	if cfg.healthCheck {
		fmt.Fprintln(os.Stderr, "Running infrastructure health checks...")
		ir, err := healthcheck.RunInfra(healthcheck.InfraConfig{DataSource: src})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: infra health checks: %v\n", err)
		} else {
			infraReport = ir
			fmt.Fprintf(os.Stderr, "Infrastructure: %d passed, %d failed, %d warnings\n",
				ir.Passed, ir.Failed, ir.Warnings)
		}
	}

	// 2. Collect symptoms for ADHD analysis
	symptoms := collectSymptoms(infraReport)

	// 2b. RAG knowledge base enrichment
	var ragContext *rca.RAGContextData
	if cfg.ragEnabled {
		ragCfgPath := cfg.ragConfigPath
		if ragCfgPath == "" {
			ragCfgPath = "rag-config.yaml"
		}
		ragCfg, ragErr := rag.LoadConfig(ragCfgPath)
		if ragErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: RAG config: %v\n", ragErr)
		} else {
			if cfg.ragDataDir != "" {
				ragCfg.DataDir = cfg.ragDataDir
			}
			ragEngine, ragErr := rag.NewEngine(ragCfg)
			if ragErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: RAG engine: %v\n", ragErr)
			} else {
				defer ragEngine.Close()
				fmt.Fprintln(os.Stderr, "Querying RAG knowledge base...")
				ragResult, ragErr := ragEngine.Troubleshoot(ctx, "cluster-infrastructure", symptoms, "")
				if ragErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: RAG lookup: %v\n", ragErr)
				} else {
					ragContext = &rca.RAGContextData{
						Summary:    ragResult.Summary,
						Confidence: ragResult.Confidence,
					}
					for _, ref := range ragResult.DocumentationRefs {
						ragContext.DocumentationRefs = append(ragContext.DocumentationRefs, rca.RAGDocRef{
							Title: ref.Title, Source: ref.Source, Excerpt: ref.Excerpt, URL: ref.URL,
						})
					}
					for _, ki := range ragResult.KnownIssues {
						ragContext.KnownIssues = append(ragContext.KnownIssues, rca.RAGKnownIssue{
							ID: ki.ID, Summary: ki.Summary, Workaround: ki.Workaround, FixVersion: ki.FixVersion,
						})
					}
					for _, ca := range ragResult.ConfigAdvice {
						ragContext.ConfigAdvice = append(ragContext.ConfigAdvice, rca.RAGConfigAdvice{
							Component: ca.Component, Reference: ca.Reference, Advice: ca.Advice,
						})
					}
					for _, ki := range ragResult.KnownIssues {
						symptoms = append(symptoms, fmt.Sprintf("[Known Issue %s] %s", ki.ID, ki.Summary))
					}
					fmt.Fprintf(os.Stderr, "RAG: %d docs, %d known issues, confidence %.0f%%\n",
						len(ragResult.DocumentationRefs), len(ragResult.KnownIssues), ragResult.Confidence*100)
				}
			}
		}
	}

	// 3. ADHD multi-frame divergent analysis
	var adhdResult *adhd.DiagnosisResult
	if cfg.adhdEnabled {
		claudeClient, err := claudeapi.NewClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: ADHD analysis requires ANTHROPIC_API_KEY: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Running ADHD divergent analysis (%d frames, depth=%s)...\n",
				cfg.frameCount, cfg.depth)

			snapshot, err := adhd.BuildClusterSnapshot(src)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: cluster snapshot: %v\n", err)
			}

			problem := "Diagnose all health issues in this OpenShift cluster"
			if len(symptoms) > 0 {
				problem = fmt.Sprintf("Diagnose cluster issues: %s", symptoms[0])
			}

			engine := adhd.NewEngine(claudeClient)
			adhdResult, err = engine.Diagnose(ctx, problem, symptoms, snapshot, adhd.DiagnosisOptions{
				FrameCount:  cfg.frameCount,
				TopK:        3,
				Depth:       cfg.depth,
				Concurrency: 4,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: ADHD analysis: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "ADHD: %d branches, %d shortlisted, %d traps identified\n",
					len(adhdResult.Branches), len(adhdResult.Shortlist), len(adhdResult.Traps))
			}
		}
	}

	// 4. Noise filtering on infra dimensions
	var noiseReport *noise.FilterReport
	if infraReport != nil {
		noiseReport = noise.Filter(env, infraReport.Dimensions)
	}

	// 5. OpenShift repo correlation
	var repoCorrelation *rca.RepoCorrelationData
	if cfg.enableRepoCorr {
		corr, err := openshift.Correlate(ctx, "cluster-infrastructure", "Cluster health check failures", openshift.CorrelateConfig{
			CacheDir:    cfg.metadataDir,
			InfraReport: infraReport,
			SkipClone:   true,
		})
		if err == nil && corr != nil {
			repoCorrelation = &rca.RepoCorrelationData{
				Operator: corr.Operator,
				RepoPath: corr.RepoPath,
				RepoURL:  corr.RepoURL,
				Classification: rca.ClassificationData{
					Type:       corr.Classification.Type,
					Confidence: corr.Classification.Confidence,
					Evidence:   corr.Classification.Evidence,
				},
				Evidence:       corr.Evidence,
				Recommendation: corr.Recommendation,
			}
		}
	}

	// 6. Self-learning from metadata store
	var similarIssues []rca.SimilarIssueData
	var learningInsights *rca.LearningInsightsData
	if cfg.enableLearning {
		metaStore, err := metadata.Open(cfg.metadataDir)
		if err == nil {
			defer metaStore.Close()
			fp := learning.BuildFingerprint(learning.SymptomInput{
				InfraReport: infraReport,
				NoiseReport: noiseReport,
			})
			rawIssues, _ := learning.FindSimilarIssues(metaStore, fp)
			for _, si := range rawIssues {
				similarIssues = append(similarIssues, rca.SimilarIssueData{
					Operator:       si.Operator,
					Classification: si.Classification,
					Resolution:     si.Resolution,
					Similarity:     si.Similarity,
					HitCount:       si.HitCount,
				})
			}
			rawInsights, _ := learning.BuildInsights(metaStore, "cluster-infrastructure", rawIssues)
			if rawInsights != nil {
				learningInsights = &rca.LearningInsightsData{}
				for _, fs := range rawInsights.FrameStats {
					learningInsights.FrameStats = append(learningInsights.FrameStats, rca.FrameStatData{
						FrameID: fs.FrameID, Total: fs.Total, Confirmed: fs.Confirmed, TrapCount: fs.TrapCount,
					})
				}
				for _, ps := range rawInsights.TopPatterns {
					learningInsights.TopPatterns = append(learningInsights.TopPatterns, rca.PatternStatData{
						Pattern: ps.Pattern, Count: ps.Count, Confidence: ps.Confidence,
					})
				}
			}
		}
	}

	// 7. Generate RCA document
	if cfg.generateRCA {
		clusterName := cfg.clusterName
		if clusterName == "" {
			clusterName = "live-cluster"
		}

		rcaInput := rca.ReportInput{
			ClusterName:      clusterName,
			Environment:      env,
			Operator:         "cluster-infrastructure",
			InfraReport:      infraReport,
			NoiseReport:      noiseReport,
			ADHDResult:       adhdResult,
			RepoCorrelation:  repoCorrelation,
			RAGContext:       ragContext,
			SimilarIssues:    similarIssues,
			LearningInsights: learningInsights,
		}
		doc := rca.GenerateDocument(rcaInput)

		if cfg.rcaFile != "" {
			if err := os.WriteFile(cfg.rcaFile, []byte(doc.Markdown), 0o600); err != nil {
				return &CLIError{Code: exitOperation, Err: fmt.Errorf("write RCA file: %w", err)}
			}
			fmt.Fprintf(os.Stderr, "RCA report written to %s\n", cfg.rcaFile)
		} else {
			fmt.Println(doc.Markdown)
		}
	}

	return nil
}

func collectSymptoms(infraReport *healthcheck.Report) []string {
	if infraReport == nil {
		return nil
	}
	symptoms := make([]string, 0)
	for _, dim := range infraReport.Dimensions {
		if dim.Status == healthcheck.StatusFail || dim.Status == healthcheck.StatusWarn {
			symptoms = append(symptoms, fmt.Sprintf("[%s] %s: %s", dim.Status, dim.Name, dim.Summary))
		}
	}
	return symptoms
}

func printDiagnoseUsage(w io.Writer) {
	const usage = `opm-diagnose — unified OpenShift cluster diagnosis

Comprehensive root-cause analysis for OpenShift clusters using multi-frame
ADHD divergent analysis. Supports both live clusters (kubeconfig) and
must-gather dumps.

Data Sources:
  --kubeconfig connects to a live cluster via the Kubernetes API
  --must-gather analyzes offline must-gather dumps
  If neither is specified, uses KUBECONFIG env var or ~/.kube/config

ADHD Analysis:
  Multi-frame divergent analysis prevents anchoring on "obvious" root causes.
  12 diagnostic frames (Network Engineer, etcd Specialist, Security Auditor,
  3AM On-Call SRE, etc.) analyze the cluster from isolated perspectives.
  Hypotheses are scored on likelihood/impact/evidence, clustered by failure
  mechanism, and traps (looks-like-the-cause-but-isn't) are flagged.

Usage:
  opm-diagnose --kubeconfig <path> [options]
  opm-diagnose --must-gather <path> [options]
  opm-diagnose [options]  # uses KUBECONFIG env or ~/.kube/config

Data Source:
  -m, --must-gather string   Must-gather directory or glob pattern
      --kubeconfig string    Kubeconfig file (default: $KUBECONFIG, ~/.kube/config)
      --context string       Kubernetes context (default: current-context)

Analysis:
  -c, --catalog string       Catalog index image for bundle analysis
  -p, --package string       Single operator (default: all faulty or telco suite)
      --version string       Target bundle version for code delta
      --telco-suite          Full telco suite (27 operators + IDMS)
      --health-check         Systematic health checks (default true)
      --environment string   Noise filter: production, disconnected, lab, kvm
      --source-repo string   Local operator source for code correlation

ADHD:
      --adhd                 Multi-frame divergent analysis (default true)
      --frames N             Divergence frames (default 5)
      --depth string         quick (3 frames), standard (5), deep (8+)

Output:
      --rca                  Generate RCA markdown (default true)
      --rca-file string      Write RCA to file
      --cluster-name string  Cluster ID for session persistence
      --state-dir string     Session state directory
      --json                 JSON output
      --timeout duration     Timeout (default 10m)

Environment:
  KUBECONFIG                 Default kubeconfig path
  DOCKER_CONFIG              Registry credentials for catalog/bundle pulls
  ANTHROPIC_API_KEY          Required for ADHD analysis and AI correlation

Examples:
  # Live cluster — full scan
  opm-diagnose --kubeconfig ~/.kube/config --rca-file /tmp/cluster-rca.md

  # Must-gather — deep ADHD analysis
  opm-diagnose --must-gather /path/to/must-gather --depth deep

  # Live cluster — telco suite focus
  opm-diagnose --kubeconfig ~/.kube/config --telco-suite --environment production

  # Must-gather — single operator with code correlation
  opm-diagnose --must-gather /path/to/mg -p kubernetes-nmstate-operator \
    --catalog quay.io/prega/prega-operator-index:v4.22-latest
`
	_, _ = fmt.Fprint(w, usage)
}
