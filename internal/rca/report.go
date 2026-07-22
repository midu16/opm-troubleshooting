package rca

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/midu16/opm-troubleshooting/internal/adhd"
	"github.com/midu16/opm-troubleshooting/internal/claudeapi"
	"github.com/midu16/opm-troubleshooting/internal/codeanalysis"
	"github.com/midu16/opm-troubleshooting/internal/gitdelta"
	"github.com/midu16/opm-troubleshooting/internal/healthcheck"
	"github.com/midu16/opm-troubleshooting/internal/imageinspect"
	"github.com/midu16/opm-troubleshooting/internal/mustgather"
	"github.com/midu16/opm-troubleshooting/internal/noise"
	"github.com/midu16/opm-troubleshooting/internal/session"
)

// Document is a professional, shareable RCA report.
type Document struct {
	Title       string    `json:"title"`
	GeneratedAt time.Time `json:"generated_at"`
	Markdown    string    `json:"markdown"`
}

// ReportInput aggregates all analysis outputs for RCA generation.
type ReportInput struct {
	ClusterName      string
	Environment      noise.Environment
	MustGatherPath   string
	Operator         string
	Namespace        string
	OperatorState    mustgather.OperatorState
	InstalledBundle  *imageinspect.BundleInfo
	TargetBundle     *imageinspect.BundleInfo
	CommitDelta      *gitdelta.CommitDelta
	ClaudeAnalysis   *claudeapi.AnalysisResponse
	RCAPatterns      []PatternMatch
	Recommendations  []AnalysisRecommendation
	HealthReport     *healthcheck.Report
	InfraReport      *healthcheck.Report
	NoiseReport      *noise.FilterReport
	CodeAnalysis     *codeanalysis.Result
	Session          *session.Record
	ADHDResult       *adhd.DiagnosisResult
	RepoCorrelation  *RepoCorrelationData
	RAGContext       *RAGContextData
	SimilarIssues    []SimilarIssueData
	LearningInsights *LearningInsightsData
}

// RepoCorrelationData is a report-layer view of OpenShift repo correlation results.
type RepoCorrelationData struct {
	Operator        string
	RepoPath        string
	RepoURL         string
	Classification  ClassificationData
	GitHubIssues    []GitHubIssueData
	RecentCommits   []CommitData
	Evidence        []string
	Recommendation  string
	BundleCommit    string
	RepoSource      string
	RepoVerified    bool
	CommitPinned    bool
	ConfidenceGrade string
}

// ClassificationData holds issue classification details for the report.
type ClassificationData struct {
	Type       string
	Confidence float64
	Evidence   []string
}

// GitHubIssueData holds GitHub issue fields for the report.
type GitHubIssueData struct {
	Number    int
	Title     string
	State     string
	URL       string
	UpdatedAt string
}

// CommitData holds git commit info for the report.
type CommitData struct {
	Hash    string
	Subject string
	Author  string
}

// SimilarIssueData holds a past similar issue for the report.
type SimilarIssueData struct {
	Operator       string
	Classification string
	Resolution     string
	Similarity     float64
	HitCount       int
}

// LearningInsightsData holds aggregated learning data for the report.
type LearningInsightsData struct {
	FrameStats  []FrameStatData
	TopPatterns []PatternStatData
}

// FrameStatData holds per-frame stats for the report.
type FrameStatData struct {
	FrameID   string
	Total     int
	Confirmed int
	TrapCount int
}

// PatternStatData holds pattern frequency for the report.
type PatternStatData struct {
	Pattern    string
	Count      int
	Confidence float64
}

// RAGContextData holds RAG knowledge base results for the report.
type RAGContextData struct {
	Summary           string
	DocumentationRefs []RAGDocRef
	KnownIssues       []RAGKnownIssue
	ConfigAdvice      []RAGConfigAdvice
	Confidence        float64

	SymptomAnalysis        []RAGSymptomEvidence
	IssueClassification    string
	ClassificationEvidence []string
	RemediationSteps       []RAGRemediationStep
	RelevantCodePaths      []RAGCodePathEvidence
}

// RAGDocRef is a documentation reference from the RAG knowledge base.
type RAGDocRef struct {
	Title   string
	Source  string
	Excerpt string
	URL     string
}

// RAGKnownIssue is a known issue from the RAG knowledge base.
type RAGKnownIssue struct {
	ID         string
	Summary    string
	Workaround string
	FixVersion string
}

// RAGConfigAdvice is a configuration recommendation from the RAG knowledge base.
type RAGConfigAdvice struct {
	Component string
	Reference string
	Advice    string
}

// RAGSymptomEvidence holds per-dimension RAG search results for the report.
type RAGSymptomEvidence struct {
	Symptom       string
	DimensionID   string
	DocMatches    []RAGDocRef
	CodeMatches   []RAGDocRef
	ConfigMatches []RAGConfigAdvice
	KnownIssues   []RAGKnownIssue
	Relevance     float64
}

// RAGRemediationStep is a prioritized remediation action from the RAG knowledge base.
type RAGRemediationStep struct {
	Step       int
	Priority   string
	Action     string
	Source     string
	Confidence float64
}

// RAGCodePathEvidence is an operator code path relevant to the failure.
type RAGCodePathEvidence struct {
	Declaration string
	FilePath    string
	Repo        string
	RepoURL     string
	Excerpt     string
	Relevance   string
}

// GenerateDocument produces a professional Markdown RCA from analysis results.
func GenerateDocument(input ReportInput) Document {
	now := time.Now().UTC()
	title := fmt.Sprintf("Root Cause Analysis: %s", input.Operator)
	if input.ClusterName != "" {
		title = fmt.Sprintf("Root Cause Analysis: %s on %s", input.Operator, input.ClusterName)
	}

	var b strings.Builder

	writeHeader(&b, title, now, input)
	writeExecutiveSummary(&b, input)
	writeEnvironment(&b, input)
	writeObservedBehavior(&b, input)
	writeHealthDimensions(&b, input)
	writeInfraHealthDimensions(&b, input)
	writeNoiseAnalysis(&b, input)
	writeRootCause(&b, input)
	if input.RAGContext != nil && len(input.RAGContext.SymptomAnalysis) > 0 {
		writeDeepAnalysis(&b, input)
	} else {
		writeRAGContext(&b, input)
	}
	writeADHDAnalysis(&b, input)
	writeSourceCodeCorrelation(&b, input)
	writeSimilarIssues(&b, input)
	writeLearningInsights(&b, input)
	writeRecommendations(&b, input)
	writeSessionContext(&b, input)
	writeAppendix(&b, input)

	return Document{
		Title:       title,
		GeneratedAt: now,
		Markdown:    b.String(),
	}
}

func writeHeader(b *strings.Builder, title string, now time.Time, input ReportInput) {
	b.WriteString(fmt.Sprintf("# %s\n\n", title))
	b.WriteString("| Field | Value |\n|-------|-------|\n")
	if input.ClusterName != "" {
		b.WriteString(fmt.Sprintf("| **Cluster** | %s |\n", input.ClusterName))
	}
	b.WriteString(fmt.Sprintf("| **Operator** | %s |\n", input.Operator))
	if input.Namespace != "" {
		b.WriteString(fmt.Sprintf("| **Namespace** | %s |\n", input.Namespace))
	}
	b.WriteString(fmt.Sprintf("| **Environment** | %s |\n", input.Environment))
	b.WriteString(fmt.Sprintf("| **Analysis Date** | %s |\n", now.Format(time.RFC3339)))
	if input.MustGatherPath != "" {
		b.WriteString(fmt.Sprintf("| **Data Source** | %s |\n", input.MustGatherPath))
	}
	if input.Session != nil {
		b.WriteString(fmt.Sprintf("| **Redeployment #** | %d |\n", input.Session.RedeploymentCount))
	}
	b.WriteString("\n")
}

func writeExecutiveSummary(b *strings.Builder, input ReportInput) {
	b.WriteString("## Executive Summary\n\n")

	summary := buildTechnicalSummary(input)
	b.WriteString(summary)
	b.WriteString("\n\n")

	severity := determineSeverity(input)
	realIssues := 0
	cosmetic := 0
	if input.NoiseReport != nil {
		realIssues = input.NoiseReport.RealIssues
		cosmetic = input.NoiseReport.CosmeticAlerts
	}
	if realIssues > 0 || cosmetic > 0 {
		b.WriteString(fmt.Sprintf("**Severity:** %s", severity))
		if realIssues > 0 {
			b.WriteString(fmt.Sprintf(" — %d real issue(s) identified", realIssues))
		}
		if cosmetic > 0 {
			b.WriteString(fmt.Sprintf(", %d cosmetic alert(s) filtered for `%s` environment", cosmetic, input.Environment))
		}
		b.WriteString(".\n")
	}

	if input.RAGContext != nil && len(input.RAGContext.KnownIssues) > 0 {
		b.WriteString(fmt.Sprintf("\nThis failure pattern matches **%d known issue(s)** in the knowledge base.\n", len(input.RAGContext.KnownIssues)))
	}
	b.WriteString("\n")
}

func writeEnvironment(b *strings.Builder, input ReportInput) {
	b.WriteString("## Environment\n\n")
	op := input.OperatorState

	version := extractClusterVersion(input)
	if version != "" {
		b.WriteString(fmt.Sprintf("- **OCP Version:** %s\n", version))
	}
	b.WriteString(fmt.Sprintf("- **Operator:** %s\n", input.Operator))
	if input.Namespace != "" {
		b.WriteString(fmt.Sprintf("- **Namespace:** %s\n", input.Namespace))
	}
	if op.Channel != "" {
		b.WriteString(fmt.Sprintf("- **Channel:** `%s`\n", op.Channel))
	}
	if op.InstalledCSV != "" {
		b.WriteString(fmt.Sprintf("- **Installed CSV:** `%s` (%s)\n", op.InstalledCSV, op.InstalledVersion))
	}
	if op.CurrentCSV != "" && op.CurrentCSV != op.InstalledCSV {
		b.WriteString(fmt.Sprintf("- **Current CSV:** `%s`\n", op.CurrentCSV))
	}
	if op.State != "" {
		b.WriteString(fmt.Sprintf("- **Subscription State:** `%s`\n", op.State))
	}
	if input.Environment != "" {
		b.WriteString(fmt.Sprintf("- **Environment:** %s\n", input.Environment))
	}
	if input.MustGatherPath != "" {
		b.WriteString(fmt.Sprintf("- **Data Source:** `%s`\n", input.MustGatherPath))
	}
	if input.InstalledBundle != nil {
		b.WriteString(fmt.Sprintf("- **Bundle Image:** %s (commit: %s)\n", input.InstalledBundle.URL, input.InstalledBundle.Commit))
	}
	b.WriteString("\n")
}

func writeObservedBehavior(b *strings.Builder, input ReportInput) {
	op := input.OperatorState

	hasConditions := len(op.Conditions) > 0
	hasWorkloadFailures := false
	hasInfraFailures := false

	if input.HealthReport != nil {
		for _, dim := range input.HealthReport.Dimensions {
			if dim.Category == "Workload" && (dim.Status == healthcheck.StatusFail || dim.Status == healthcheck.StatusWarn) {
				hasWorkloadFailures = true
				break
			}
		}
	}
	if input.InfraReport != nil && input.InfraReport.Failed > 0 {
		hasInfraFailures = true
	}

	if !hasConditions && !hasWorkloadFailures && !hasInfraFailures && !op.Faulty {
		return
	}

	b.WriteString("## Observed Behavior\n\n")

	// OLM Lifecycle State
	b.WriteString("### OLM Lifecycle State\n\n")
	b.WriteString("```\n")
	b.WriteString(fmt.Sprintf("Subscription: %s\n", op.PackageName))
	stateDisplay := op.State
	if stateDisplay == "" {
		stateDisplay = "<none>"
	}
	b.WriteString(fmt.Sprintf("State: %s\n", stateDisplay))
	if op.Channel != "" {
		b.WriteString(fmt.Sprintf("Channel: %s\n", op.Channel))
	}
	if op.InstalledCSV != "" {
		b.WriteString(fmt.Sprintf("Installed CSV: %s\n", op.InstalledCSV))
	} else if op.Faulty || (input.HealthReport != nil && input.HealthReport.Failed > 0) {
		b.WriteString("Installed CSV: <none>\n")
	}
	if op.FailureReason != "" {
		b.WriteString(fmt.Sprintf("Failure Reason: %s\n", op.FailureReason))
	}
	for _, cond := range op.Conditions {
		if cond.Status == "True" {
			b.WriteString(fmt.Sprintf("Condition: %s = %s\n", cond.Type, cond.Status))
			if cond.Message != "" {
				msg := cond.Message
				if len(msg) > 300 {
					msg = msg[:300] + "..."
				}
				b.WriteString(fmt.Sprintf("  Message: %s\n", msg))
			}
		}
	}
	b.WriteString("```\n\n")

	// OLM dimension evidence
	if input.HealthReport != nil {
		for _, dim := range input.HealthReport.Dimensions {
			if dim.Category == "OLM" && (dim.Status == healthcheck.StatusFail || dim.Status == healthcheck.StatusWarn) {
				b.WriteString(fmt.Sprintf("**%s** [%s]: %s\n\n", dim.Name, dim.Status, dim.Summary))
				if len(dim.Evidence) > 0 {
					renderEvidenceBlock(b, dim.Evidence, 5)
					b.WriteString("\n")
				}
			}
		}
	}

	// Workload State
	if hasWorkloadFailures {
		b.WriteString("### Workload State\n\n")
		for _, dim := range input.HealthReport.Dimensions {
			if dim.Category == "Workload" && (dim.Status == healthcheck.StatusFail || dim.Status == healthcheck.StatusWarn) {
				b.WriteString(fmt.Sprintf("**%s** [%s]: %s\n\n", dim.Name, dim.Status, dim.Summary))
				if len(dim.Evidence) > 0 {
					renderEvidenceBlock(b, dim.Evidence, 10)
					b.WriteString("\n")
				}
			}
		}
	}

	// Infrastructure State
	if hasInfraFailures {
		b.WriteString("### Infrastructure State\n\n")
		for _, dim := range input.InfraReport.Dimensions {
			if dim.Status == healthcheck.StatusFail || dim.Status == healthcheck.StatusWarn {
				b.WriteString(fmt.Sprintf("**%s** [%s]: %s\n\n", dim.Name, dim.Status, dim.Summary))
				if len(dim.Evidence) > 0 {
					renderEvidenceBlock(b, dim.Evidence, 10)
					b.WriteString("\n")
				}
			}
		}
	}
}

func writeHealthDimensions(b *strings.Builder, input ReportInput) {
	if input.HealthReport == nil {
		return
	}
	hr := input.HealthReport
	b.WriteString(fmt.Sprintf("## OLM Health Check (%d Dimensions)\n\n", hr.TotalDimensions))
	b.WriteString(fmt.Sprintf(
		"**Result:** %d passed, %d failed, %d warnings, %d skipped (of %d dimensions)\n\n",
		hr.Passed, hr.Failed, hr.Warnings, hr.Skipped, hr.TotalDimensions,
	))

	b.WriteString("| # | Dimension | Category | Status | Severity | Summary |\n")
	b.WriteString("|---|-----------|----------|--------|----------|----------|\n")
	for i, dim := range hr.Dimensions {
		b.WriteString(fmt.Sprintf(
			"| %d | %s | %s | %s | %s | %s |\n",
			i+1, dim.Name, dim.Category, dim.Status, dim.Severity, truncate(dim.Summary, 80),
		))
		if (dim.Status == healthcheck.StatusFail || dim.Status == healthcheck.StatusWarn) && len(dim.Evidence) > 0 {
			for _, e := range dim.Evidence {
				b.WriteString(fmt.Sprintf("|  |  |  |  |  | --> %s |\n", truncate(e, 100)))
			}
		}
	}
	b.WriteString("\n")
}

func writeInfraHealthDimensions(b *strings.Builder, input ReportInput) {
	if input.InfraReport == nil {
		return
	}
	ir := input.InfraReport
	b.WriteString(fmt.Sprintf("## Infrastructure Health Check (%d Dimensions)\n\n", ir.TotalDimensions))
	b.WriteString(fmt.Sprintf(
		"**Result:** %d passed, %d failed, %d warnings, %d skipped (of %d dimensions)\n\n",
		ir.Passed, ir.Failed, ir.Warnings, ir.Skipped, ir.TotalDimensions,
	))

	b.WriteString("| # | Dimension | Category | Status | Severity | Summary |\n")
	b.WriteString("|---|-----------|----------|--------|----------|----------|\n")
	for i, dim := range ir.Dimensions {
		b.WriteString(fmt.Sprintf(
			"| %d | %s | %s | %s | %s | %s |\n",
			i+1, dim.Name, dim.Category, dim.Status, dim.Severity, truncate(dim.Summary, 80),
		))
		if (dim.Status == healthcheck.StatusFail || dim.Status == healthcheck.StatusWarn) && len(dim.Evidence) > 0 {
			for _, e := range dim.Evidence {
				b.WriteString(fmt.Sprintf("|  |  |  |  |  | --> %s |\n", truncate(e, 100)))
			}
		}
	}
	b.WriteString("\n")
}

func writeADHDAnalysis(b *strings.Builder, input ReportInput) {
	if input.ADHDResult == nil {
		return
	}
	ar := input.ADHDResult

	b.WriteString("## Divergent Analysis (ADHD Framework)\n\n")
	b.WriteString(fmt.Sprintf("**Problem statement:** %s\n\n", ar.Problem))

	writeDivergentSummary(b, ar)
	writeHypothesisTable(b, ar)
	writeTrapIdentification(b, ar)
	writeEvidenceChains(b, ar)
	writeNonObviousFinding(b, ar)
	writeDeepDive(b, ar)
	writeProvocation(b, ar)
}

func writeDivergentSummary(b *strings.Builder, ar *adhd.DiagnosisResult) {
	if len(ar.Branches) == 0 {
		return
	}
	b.WriteString("### Divergent Analysis Summary\n\n")
	b.WriteString(fmt.Sprintf("%d cognitive frames generated hypotheses in isolation:\n\n", len(ar.Branches)))

	b.WriteString("| Frame | Hypotheses | Top Finding |\n")
	b.WriteString("|-------|------------|-------------|\n")
	for _, branch := range ar.Branches {
		top := "-"
		if len(branch.Hypotheses) > 0 {
			top = truncate(branch.Hypotheses[0].Text, 70)
		}
		b.WriteString(fmt.Sprintf("| %s | %d | %s |\n",
			branch.FrameName, len(branch.Hypotheses), top))
	}
	b.WriteString("\n")
}

func writeHypothesisTable(b *strings.Builder, ar *adhd.DiagnosisResult) {
	if len(ar.Shortlist) == 0 {
		return
	}
	b.WriteString("### Hypothesis Scoring\n\n")
	b.WriteString("Top hypotheses ranked by composite score (Likelihood×0.40 + Impact×0.25 + Evidence×0.35):\n\n")

	b.WriteString("| Rank | Hypothesis | Likelihood | Impact | Evidence | Total | Trap? |\n")
	b.WriteString("|------|-----------|------------|--------|----------|-------|-------|\n")
	for i, h := range ar.Shortlist {
		trap := "-"
		if h.Score.Trap != "" {
			trap = truncate(h.Score.Trap, 30)
		}
		b.WriteString(fmt.Sprintf("| %d | %s | %.1f | %.1f | %.1f | **%.2f** | %s |\n",
			i+1, truncate(h.Text, 50),
			h.Score.Likelihood, h.Score.Impact, h.Score.Evidence, h.Score.Total, trap))
	}
	b.WriteString("\n")
}

func writeTrapIdentification(b *strings.Builder, ar *adhd.DiagnosisResult) {
	if len(ar.Traps) == 0 {
		return
	}
	b.WriteString("### Trap Identification\n\n")
	b.WriteString("These findings look like root causes but are actually symptoms or mitigations:\n\n")
	for _, t := range ar.Traps {
		b.WriteString(fmt.Sprintf("- **%s** — %s\n", truncate(t.Text, 60), t.Score.Trap))
	}
	b.WriteString("\n")
}

func writeEvidenceChains(b *strings.Builder, ar *adhd.DiagnosisResult) {
	if len(ar.Shortlist) == 0 {
		return
	}
	hasEvidence := false
	for _, h := range ar.Shortlist {
		if len(h.Evidence) > 0 {
			hasEvidence = true
			break
		}
	}
	if !hasEvidence {
		return
	}

	b.WriteString("### Evidence Chains\n\n")
	for i, h := range ar.Shortlist {
		if len(h.Evidence) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("**#%d: %s** (frame: %s, score: %.2f)\n\n",
			i+1, truncate(h.Text, 70), h.FrameID, h.Score.Total))
		b.WriteString(fmt.Sprintf("*Rationale:* %s\n\n", h.Rationale))
		b.WriteString("Evidence:\n")
		for _, e := range h.Evidence {
			b.WriteString(fmt.Sprintf("- %s\n", e))
		}
		b.WriteString("\n")
	}
}

func writeNonObviousFinding(b *strings.Builder, ar *adhd.DiagnosisResult) {
	if ar.NonObvious == nil {
		return
	}
	no := ar.NonObvious
	b.WriteString("### Non-Obvious Finding\n\n")
	b.WriteString(fmt.Sprintf("> **%s** (score: %.2f)\n>\n", no.Text, no.Score.Total))
	b.WriteString(fmt.Sprintf("> *Frame:* %s | *Rationale:* %s\n\n", no.FrameID, no.Rationale))
	if len(no.Evidence) > 0 {
		b.WriteString("Supporting evidence:\n")
		for _, e := range no.Evidence {
			b.WriteString(fmt.Sprintf("- %s\n", e))
		}
		b.WriteString("\n")
	}
}

func writeDeepDive(b *strings.Builder, ar *adhd.DiagnosisResult) {
	if len(ar.Deepened) == 0 {
		return
	}
	b.WriteString("### Technical Deep Dives\n\n")
	for i, d := range ar.Deepened {
		b.WriteString(fmt.Sprintf("#### Deep Dive #%d\n\n", i+1))
		b.WriteString(fmt.Sprintf("**Investigation sketch:** %s\n\n", d.Sketch))
		b.WriteString(fmt.Sprintf("**Load-bearing risk:** %s\n\n", d.LoadBearingRisk))
		b.WriteString(fmt.Sprintf("**Recommended first step:** %s\n\n", d.FirstStep))
		if len(d.ChildHypotheses) > 0 {
			b.WriteString("Sub-hypotheses to investigate:\n")
			for _, ch := range d.ChildHypotheses {
				b.WriteString(fmt.Sprintf("- %s (likelihood: %.1f)\n", ch.Text, ch.Score.Likelihood))
			}
			b.WriteString("\n")
		}
	}
}

func writeProvocation(b *strings.Builder, ar *adhd.DiagnosisResult) {
	if ar.Provocation == "" {
		return
	}
	b.WriteString("### Provocation\n\n")
	b.WriteString(fmt.Sprintf("> %s\n\n", ar.Provocation))
}

func writeNoiseAnalysis(b *strings.Builder, input ReportInput) {
	if input.NoiseReport == nil || input.NoiseReport.TotalFindings == 0 {
		return
	}
	nr := input.NoiseReport
	b.WriteString("## Noise Filtering\n\n")
	b.WriteString(fmt.Sprintf(
		"Environment profile: **%s** — %d real, %d cosmetic, %d ambiguous\n\n",
		nr.Environment, nr.RealIssues, nr.CosmeticAlerts, nr.AmbiguousAlerts,
	))

	if len(nr.Findings) > 0 {
		b.WriteString("| Classification | Dimension | Reason |\n")
		b.WriteString("|----------------|-----------|--------|\n")
		for _, f := range nr.Findings {
			reason := f.NoiseReason
			if reason == "" {
				reason = f.Dimension.Summary
			}
			b.WriteString(fmt.Sprintf(
				"| %s | %s | %s |\n",
				f.Classification, f.Dimension.Name, truncate(reason, 100),
			))
		}
		b.WriteString("\n")
	}
}

func writeRootCause(b *strings.Builder, input ReportInput) {
	b.WriteString("## Root Cause Analysis\n\n")
	op := input.OperatorState

	if op.FailureReason != "" {
		b.WriteString(fmt.Sprintf("**Primary failure:** %s\n\n", op.FailureReason))
	}

	if op.RootCause != nil {
		rc := op.RootCause
		if len(rc.MissingCRDs) > 0 {
			b.WriteString(fmt.Sprintf("- **Missing CRDs:** %s\n", strings.Join(rc.MissingCRDs, ", ")))
		}
		if len(rc.UnknownResources) > 0 {
			b.WriteString(fmt.Sprintf("- **Unknown resources:** %s\n", strings.Join(rc.UnknownResources, ", ")))
		}
		if len(rc.NotPresentResources) > 0 {
			b.WriteString(fmt.Sprintf("- **Not present:** %s\n", strings.Join(rc.NotPresentResources, ", ")))
		}
		if rc.RawFailureMessage != "" {
			b.WriteString(fmt.Sprintf("- **InstallPlan failure:** %s\n", rc.RawFailureMessage))
		}
		b.WriteString("\n")
	}

	if len(input.RCAPatterns) > 0 {
		b.WriteString("### RCA Pattern Detection\n\n")
		for _, p := range input.RCAPatterns {
			b.WriteString(fmt.Sprintf(
				"- **%s** (confidence %.0f%%): %s\n",
				p.Pattern, p.Confidence*100, p.Description,
			))
			if len(p.Evidence) > 0 {
				b.WriteString(fmt.Sprintf("  - Evidence: %s\n", strings.Join(p.Evidence, ", ")))
			}
		}
		b.WriteString("\n")
	}

	if input.ClaudeAnalysis != nil {
		ca := input.ClaudeAnalysis
		if ca.Summary != "" {
			b.WriteString(fmt.Sprintf("### AI Code Change Correlation\n\n%s\n\n", ca.Summary))
		}
		if len(ca.LikelyCauses) > 0 {
			b.WriteString("**Likely causes from code delta:**\n")
			for _, c := range ca.LikelyCauses {
				b.WriteString(fmt.Sprintf("- %s\n", c))
			}
			b.WriteString("\n")
		}
	}
}

func writeRecommendations(b *strings.Builder, input ReportInput) {
	b.WriteString("## Recommended Actions\n\n")
	step := 1

	if input.HealthReport != nil {
		for _, dim := range input.HealthReport.Dimensions {
			if dim.Status == healthcheck.StatusFail && dim.Recommendation != "" {
				b.WriteString(fmt.Sprintf("%d. **[%s]** %s\n", step, dim.Name, dim.Recommendation))
				step++
			}
		}
	}

	if input.InfraReport != nil {
		for _, dim := range input.InfraReport.Dimensions {
			if dim.Status == healthcheck.StatusFail && dim.Recommendation != "" {
				b.WriteString(fmt.Sprintf("%d. **[%s]** %s\n", step, dim.Name, dim.Recommendation))
				step++
			}
		}
	}

	for _, rec := range input.Recommendations {
		level := "Medium"
		switch rec.Priority {
		case 1:
			level = "Critical"
		case 2:
			level = "High"
		}
		b.WriteString(fmt.Sprintf("%d. **[%s]** %s — %s\n", step, level, rec.Title, rec.Description))
		step++
	}

	if input.ClaudeAnalysis != nil {
		for _, action := range input.ClaudeAnalysis.RecommendedActions {
			b.WriteString(fmt.Sprintf("%d. %s\n", step, action))
			step++
		}
	}

	if step == 1 {
		b.WriteString("No immediate actions required. Continue monitoring.\n")
	}
	b.WriteString("\n")
}

func writeSessionContext(b *strings.Builder, input ReportInput) {
	if input.Session == nil || len(input.Session.History) == 0 {
		return
	}
	b.WriteString("## Deployment History\n\n")
	b.WriteString("Context maintained across redeployments:\n\n")
	for _, entry := range input.Session.History {
		b.WriteString(fmt.Sprintf(
			"- **%s** (redeploy #%d): %s — %s\n",
			entry.Timestamp.Format("2006-01-02 15:04"),
			entry.RedeploymentNum,
			entry.Summary,
			entry.Status,
		))
	}
	b.WriteString("\n")

	if len(input.Session.KnownCosmetic) > 0 {
		b.WriteString("### Known Cosmetic Alerts (suppressed)\n\n")
		for _, c := range input.Session.KnownCosmetic {
			b.WriteString(fmt.Sprintf("- %s\n", c))
		}
		b.WriteString("\n")
	}
}

func writeSourceCodeCorrelation(b *strings.Builder, input ReportInput) {
	if input.RepoCorrelation == nil {
		return
	}
	rc := input.RepoCorrelation

	b.WriteString("## Source Code Correlation\n\n")

	// Repository Provenance
	b.WriteString("### Repository Provenance\n\n")
	b.WriteString("| Field | Value |\n|-------|-------|\n")
	b.WriteString(fmt.Sprintf("| **Repository** | [%s](%s) |\n", rc.RepoPath, rc.RepoURL))

	sourceLabel := rc.RepoSource
	switch rc.RepoSource {
	case "static_registry":
		sourceLabel = "Static registry (known operator mapping)"
	case "bundle_labels":
		sourceLabel = "Bundle image labels"
	case "bundle_csv":
		sourceLabel = "Bundle CSV annotations"
	case "inferred":
		sourceLabel = "Inferred from package name"
	}
	b.WriteString(fmt.Sprintf("| **Discovery Method** | %s |\n", sourceLabel))

	verifiedLabel := "No"
	if rc.RepoVerified {
		verifiedLabel = "Yes"
	}
	b.WriteString(fmt.Sprintf("| **GitHub Verified** | %s |\n", verifiedLabel))

	if rc.ConfidenceGrade != "" {
		b.WriteString(fmt.Sprintf("| **Confidence Grade** | **%s** (%.0f%%) |\n",
			rc.ConfidenceGrade, rc.Classification.Confidence*100))
	}
	b.WriteString("\n")

	// Commit-Pinned Analysis
	b.WriteString("### Commit Analysis\n\n")
	switch {
	case rc.CommitPinned && rc.BundleCommit != "":
		commitShort := rc.BundleCommit
		if len(commitShort) > 12 {
			commitShort = commitShort[:12]
		}
		b.WriteString(fmt.Sprintf("Analysis pinned to deployed commit `%s`. ", commitShort))
		b.WriteString("This ensures code correlation reflects the exact version running in the cluster, not the latest upstream changes.\n\n")
	case rc.BundleCommit != "":
		commitShort := rc.BundleCommit
		if len(commitShort) > 12 {
			commitShort = commitShort[:12]
		}
		b.WriteString(fmt.Sprintf("Deployed commit: `%s` (analysis performed at HEAD — commit-pinned analysis was unavailable).\n\n", commitShort))
	default:
		b.WriteString("No deployed commit hash available. Analysis performed at repository HEAD.\n\n")
	}

	// Classification
	b.WriteString("### Configuration vs Code Classification\n\n")
	b.WriteString(fmt.Sprintf("**Classification:** **%s** (confidence: %.0f%%)\n\n",
		rc.Classification.Type, rc.Classification.Confidence*100))

	if len(rc.Classification.Evidence) > 0 {
		b.WriteString("**Evidence chain:**\n")
		for _, e := range rc.Classification.Evidence {
			b.WriteString(fmt.Sprintf("- %s\n", e))
		}
		b.WriteString("\n")
	}

	if rc.Recommendation != "" {
		b.WriteString(fmt.Sprintf("**Recommendation:** %s\n\n", rc.Recommendation))
	}

	// ADHD Critical Thinking Integration
	if input.ADHDResult != nil {
		for _, branch := range input.ADHDResult.Branches {
			if branch.FrameID == "source-code-forensics" && len(branch.Hypotheses) > 0 {
				b.WriteString("### Source Code Forensics (ADHD Frame)\n\n")
				for i, h := range branch.Hypotheses {
					if i >= 3 {
						break
					}
					b.WriteString(fmt.Sprintf("- **%s** (score: %.2f)\n", truncate(h.Text, 80), h.Score.Total))
					if h.Rationale != "" {
						b.WriteString(fmt.Sprintf("  - %s\n", truncate(h.Rationale, 120)))
					}
				}
				b.WriteString("\n")
				break
			}
		}
	}

	// GitHub Issues
	if len(rc.GitHubIssues) > 0 {
		b.WriteString("### Related GitHub Issues\n\n")
		b.WriteString("| # | Title | State | Updated |\n")
		b.WriteString("|---|-------|-------|---------|\n")
		for i, issue := range rc.GitHubIssues {
			if i >= 10 {
				break
			}
			b.WriteString(fmt.Sprintf("| [#%d](%s) | %s | %s | %s |\n",
				issue.Number, issue.URL, truncate(issue.Title, 60), issue.State,
				issue.UpdatedAt))
		}
		b.WriteString("\n")
	}

	// Recent Commits
	if len(rc.RecentCommits) > 0 {
		b.WriteString("### Recent Upstream Changes\n\n")
		for i, c := range rc.RecentCommits {
			if i >= 10 {
				break
			}
			hashLen := len(c.Hash)
			if hashLen > 8 {
				hashLen = 8
			}
			b.WriteString(fmt.Sprintf("- `%s` %s (%s)\n", c.Hash[:hashLen], c.Subject, c.Author))
		}
		b.WriteString("\n")
	}
}

func writeRAGContext(b *strings.Builder, input ReportInput) {
	if input.RAGContext == nil {
		return
	}
	rag := input.RAGContext

	b.WriteString("## Knowledge Base Evidence\n\n")

	if rag.Confidence > 0 {
		b.WriteString(fmt.Sprintf("**RAG Confidence**: %.0f%%\n\n", rag.Confidence*100))
	}

	if len(rag.KnownIssues) > 0 {
		b.WriteString("### Known Issues\n\n")
		b.WriteString("| ID | Summary | Workaround | Fix Version |\n")
		b.WriteString("|-----|---------|------------|-------------|\n")
		for _, ki := range rag.KnownIssues {
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				ki.ID, ki.Summary, ki.Workaround, ki.FixVersion))
		}
		b.WriteString("\n")
	}

	if len(rag.DocumentationRefs) > 0 {
		b.WriteString("### Relevant Documentation\n\n")
		b.WriteString("| Source | Title | Excerpt |\n")
		b.WriteString("|--------|-------|--------|\n")
		for _, ref := range rag.DocumentationRefs {
			excerpt := ref.Excerpt
			if len(excerpt) > 120 {
				excerpt = excerpt[:120] + "..."
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %s |\n",
				ref.Source, ref.Title, excerpt))
		}
		b.WriteString("\n")
	}

	if len(rag.ConfigAdvice) > 0 {
		b.WriteString("### Configuration Recommendations\n\n")
		for _, ca := range rag.ConfigAdvice {
			b.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", ca.Component, ca.Reference, ca.Advice))
		}
		b.WriteString("\n")
	}
}

func writeDeepAnalysis(b *strings.Builder, input ReportInput) {
	ragCtx := input.RAGContext
	classification := ragCtx.IssueClassification

	b.WriteString("## Deep Analysis: Root Cause Determination\n\n")

	// Section 1: Failure Chain Analysis
	b.WriteString("### Failure Chain Analysis\n\n")
	b.WriteString("The following analysis traces the causal chain from the initial symptom to the root cause, using evidence from the must-gather data correlated with OpenShift documentation and known issues.\n\n")

	failedDims := failedDimensionsInCausalOrder(input)
	symByDim := make(map[string]RAGSymptomEvidence)
	for _, se := range ragCtx.SymptomAnalysis {
		symByDim[se.DimensionID] = se
	}

	for i, dim := range failedDims {
		statusTag := "FAIL"
		if dim.Status == healthcheck.StatusWarn {
			statusTag = "WARN"
		}
		b.WriteString(fmt.Sprintf("#### Step %d: %s [%s]\n\n", i+1, dim.Name, statusTag))

		b.WriteString(fmt.Sprintf("The %s check reports: **%s**\n\n", dim.Name, dim.Summary))

		if len(dim.Evidence) > 0 {
			b.WriteString("**Evidence from must-gather:**\n\n")
			renderEvidenceBlock(b, dim.Evidence, 8)
			b.WriteString("\n")
		}

		if dim.Recommendation != "" {
			b.WriteString(fmt.Sprintf("**Recommendation:** %s\n\n", dim.Recommendation))
		}

		if se, ok := symByDim[string(dim.ID)]; ok {
			if len(se.DocMatches) > 0 && se.Relevance >= 0.4 {
				best := se.DocMatches[0]
				b.WriteString(fmt.Sprintf("**Supporting Documentation** (relevance: %.0f%%):\n", se.Relevance*100))
				excerpt := best.Excerpt
				if len(excerpt) > 400 {
					excerpt = excerpt[:400] + "..."
				}
				excerpt = strings.ReplaceAll(excerpt, "\n", "\n> ")
				b.WriteString(fmt.Sprintf("> **%s** — %s\n>\n> %s\n\n", best.Title, best.Source, excerpt))
				if len(se.DocMatches) > 1 {
					b.WriteString(fmt.Sprintf("*+%d additional documentation matches*\n\n", len(se.DocMatches)-1))
				}
			}

			if len(se.CodeMatches) > 0 {
				best := se.CodeMatches[0]
				b.WriteString(fmt.Sprintf("**Relevant Code Path:** `%s` in `%s`\n\n", best.Title, best.Source))
				if best.Excerpt != "" {
					codeExcerpt := best.Excerpt
					if len(codeExcerpt) > 300 {
						codeExcerpt = codeExcerpt[:300] + "..."
					}
					b.WriteString("```go\n")
					b.WriteString(codeExcerpt)
					b.WriteString("\n```\n\n")
				}
			}

			if len(se.KnownIssues) > 0 {
				ki := se.KnownIssues[0]
				b.WriteString(fmt.Sprintf("**Known Issue:** %s — %s\n", ki.ID, ki.Summary))
				if ki.Workaround != "" {
					b.WriteString(fmt.Sprintf("- **Workaround:** %s\n", ki.Workaround))
				}
				if ki.FixVersion != "" {
					b.WriteString(fmt.Sprintf("- **Fixed in:** %s\n", ki.FixVersion))
				}
				b.WriteString("\n")
			}
		}

		sig := dimensionSignificance(dim, classification)
		b.WriteString(fmt.Sprintf("**Significance:** %s\n\n", sig))
		b.WriteString("---\n\n")
	}

	// Section 2: Root Cause Determination
	b.WriteString("### Root Cause Determination\n\n")
	if classification != "" && classification != "unknown" {
		b.WriteString(fmt.Sprintf("**Verdict:** This is a **%s** issue (RAG Confidence: %.0f%%).\n\n",
			strings.ToUpper(classification), ragCtx.Confidence*100))
	} else {
		b.WriteString(fmt.Sprintf("**Verdict:** Root cause classification is **inconclusive** (RAG Confidence: %.0f%%). Additional evidence from operator code, known issues, and configuration references would improve determination.\n\n",
			ragCtx.Confidence*100))
	}

	if len(ragCtx.ClassificationEvidence) > 0 {
		b.WriteString("**Supporting evidence:**\n\n")
		for i, e := range ragCtx.ClassificationEvidence {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, e))
		}
		b.WriteString("\n")
	}

	for _, p := range input.RCAPatterns {
		b.WriteString(fmt.Sprintf("- **Pattern detected:** %s (confidence: %.0f%%) — %s\n",
			p.Pattern, p.Confidence*100, p.Description))
	}
	if len(input.RCAPatterns) > 0 {
		b.WriteString("\n")
	}

	// Core Issue synthesis
	b.WriteString("**Core Issue:**\n\n")
	coreIssue := buildCoreIssueSynthesis(input, failedDims, classification)
	b.WriteString(coreIssue)
	b.WriteString("\n\n")

	// Section 3: Impact Assessment
	writeImpactAssessment(b, input, failedDims)

	// Section 4: Workaround and Remediation
	writeWorkaroundRemediation(b, input)

	// Section 5: References
	writeDeepReferences(b, input)
}

func buildCoreIssueSynthesis(input ReportInput, failedDims []healthcheck.DimensionResult, classification string) string {
	op := input.OperatorState
	var parts []string

	if len(failedDims) == 0 {
		return fmt.Sprintf("The %s operator shows no critical failures. Monitor the warned dimensions for potential escalation.", op.PackageName)
	}

	primary := failedDims[0]
	switch {
	case classification == "configuration":
		parts = append(parts, fmt.Sprintf("The %s operator failure is caused by a configuration issue.", op.PackageName))
		if primary.ID == healthcheck.DimSubscription || primary.ID == healthcheck.DimCatalogSource {
			parts = append(parts, fmt.Sprintf("The OLM subscription cannot resolve because %s.", primary.Summary))
			if op.Channel != "" {
				parts = append(parts, fmt.Sprintf("The operator is configured on channel `%s` — verify that the CatalogSource contains this package and that all dependency operators are available.", op.Channel))
			}
		} else {
			parts = append(parts, fmt.Sprintf("The primary failure is in %s: %s.", primary.Name, primary.Summary))
		}

	case classification == "infrastructure":
		parts = append(parts, fmt.Sprintf("The %s operator failure is caused by underlying infrastructure degradation.", op.PackageName))
		for _, dim := range failedDims {
			if dim.Category == "Infrastructure" {
				parts = append(parts, fmt.Sprintf("Infrastructure issue: %s — %s.", dim.Name, dim.Summary))
				break
			}
		}
		parts = append(parts, "Resolve the infrastructure issue before investigating operator-level failures, as they may be symptoms of the underlying platform problem.")

	case classification == "code":
		parts = append(parts, fmt.Sprintf("The %s operator failure appears to be a code-level issue.", op.PackageName))
		if len(input.RAGContext.KnownIssues) > 0 {
			ki := input.RAGContext.KnownIssues[0]
			parts = append(parts, fmt.Sprintf("This matches known issue %s: %s.", ki.ID, ki.Summary))
		}

	default:
		parts = append(parts, fmt.Sprintf("The %s operator has %d failing health dimension(s).", op.PackageName, len(failedDims)))
		parts = append(parts, fmt.Sprintf("The primary failure is in %s (%s): %s.", primary.Name, primary.Category, primary.Summary))
		hasInfra := false
		for _, dim := range failedDims {
			if dim.Category == "Infrastructure" {
				hasInfra = true
				break
			}
		}
		if hasInfra {
			parts = append(parts, "Infrastructure failures are also present, which may be contributing to operator-level issues.")
		}
	}

	return strings.Join(parts, " ")
}

func writeImpactAssessment(b *strings.Builder, input ReportInput, failedDims []healthcheck.DimensionResult) {
	if len(failedDims) == 0 {
		return
	}

	b.WriteString("### Impact Assessment\n\n")

	hasInfraFailure := false
	hasOLMFailure := false
	hasWorkloadFailure := false
	for _, dim := range failedDims {
		switch dim.Category {
		case "Infrastructure":
			hasInfraFailure = true
		case "OLM":
			hasOLMFailure = true
		case "Workload":
			hasWorkloadFailure = true
		}
	}

	switch {
	case hasInfraFailure && (hasOLMFailure || hasWorkloadFailure):
		b.WriteString("**Blast radius:** This issue has **mixed scope** — infrastructure degradation may be contributing to operator failures across multiple namespaces.\n\n")
	case hasInfraFailure:
		b.WriteString("**Blast radius:** This is a **cluster-wide** infrastructure issue that may affect multiple operators and workloads.\n\n")
	default:
		b.WriteString(fmt.Sprintf("**Blast radius:** This issue is **operator-scoped**, affecting only the `%s` deployment in namespace `%s`.\n\n",
			input.Operator, input.Namespace))
	}

	b.WriteString("**Affected components:**\n\n")
	for _, dim := range failedDims {
		b.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", dim.Name, dim.Category, dim.Summary))
	}
	b.WriteString("\n")

	if input.InfraReport != nil {
		var healthy []string
		for _, dim := range input.InfraReport.Dimensions {
			if dim.Status == healthcheck.StatusPass {
				healthy = append(healthy, dim.Name)
			}
		}
		if len(healthy) > 0 {
			b.WriteString(fmt.Sprintf("**Healthy subsystems:** %s\n\n", strings.Join(healthy, ", ")))
		}
	}
}

func writeWorkaroundRemediation(b *strings.Builder, input ReportInput) {
	ragCtx := input.RAGContext
	hasWorkaround := false
	hasFix := false

	if ragCtx != nil {
		for _, ki := range ragCtx.KnownIssues {
			if ki.Workaround != "" {
				hasWorkaround = true
				break
			}
		}
		for _, rs := range ragCtx.RemediationSteps {
			if rs.Priority == "Critical" || rs.Priority == "High" {
				hasFix = true
				break
			}
		}
	}

	failedDims := failedDimensionsInCausalOrder(input)
	for _, dim := range failedDims {
		if dim.Recommendation != "" {
			hasFix = true
			break
		}
	}

	if !hasWorkaround && !hasFix {
		return
	}

	// Immediate Workaround
	if hasWorkaround {
		b.WriteString("### Immediate Workaround\n\n")
		for _, ki := range ragCtx.KnownIssues {
			if ki.Workaround == "" {
				continue
			}
			b.WriteString(fmt.Sprintf("**Known Issue %s:** %s\n\n", ki.ID, ki.Summary))
			b.WriteString("```\n")
			b.WriteString(ki.Workaround)
			b.WriteString("\n```\n\n")
			if ki.FixVersion != "" {
				b.WriteString(fmt.Sprintf("*Permanent fix available in version %s.*\n\n", ki.FixVersion))
			}
		}
	}

	// Suggested Fix
	b.WriteString("### Suggested Fix\n\n")
	step := 1

	for _, dim := range failedDims {
		if dim.Recommendation != "" {
			b.WriteString(fmt.Sprintf("%d. **%s:** %s\n", step, dim.Name, dim.Recommendation))
			step++
		}
	}

	if ragCtx != nil {
		for _, ca := range ragCtx.ConfigAdvice {
			b.WriteString(fmt.Sprintf("%d. **Validate %s configuration:** %s (%s)\n", step, ca.Component, truncate(ca.Advice, 200), ca.Reference))
			step++
			if step > 8 {
				break
			}
		}
	}

	if step == 1 {
		b.WriteString("No specific fix guidance available. Review the failure chain analysis above and consult the referenced documentation.\n")
	}
	b.WriteString("\n")
}

func writeDeepReferences(b *strings.Builder, input ReportInput) {
	ragCtx := input.RAGContext
	hasSources := ragCtx != nil && len(ragCtx.RelevantCodePaths) > 0
	hasDocs := ragCtx != nil && len(ragCtx.DocumentationRefs) > 0
	hasIssues := ragCtx != nil && len(ragCtx.KnownIssues) > 0

	if !hasSources && !hasDocs && !hasIssues {
		return
	}

	b.WriteString("### References\n\n")

	if hasSources {
		b.WriteString("**Source Files:**\n\n")
		for i, cp := range ragCtx.RelevantCodePaths {
			if i >= 10 {
				b.WriteString(fmt.Sprintf("- ... and %d more source files\n", len(ragCtx.RelevantCodePaths)-10))
				break
			}
			if cp.Repo != "" {
				b.WriteString(fmt.Sprintf("- `%s` — %s (%s)\n", cp.FilePath, cp.Declaration, cp.Repo))
			} else {
				b.WriteString(fmt.Sprintf("- `%s` — %s\n", cp.FilePath, cp.Declaration))
			}
		}
		b.WriteString("\n")
	}

	if hasDocs {
		b.WriteString("**Documentation:**\n\n")
		for i, ref := range ragCtx.DocumentationRefs {
			if i >= 10 {
				b.WriteString(fmt.Sprintf("- ... and %d more references\n", len(ragCtx.DocumentationRefs)-10))
				break
			}
			if ref.URL != "" {
				b.WriteString(fmt.Sprintf("- [%s](%s) — %s\n", ref.Title, ref.URL, ref.Source))
			} else {
				b.WriteString(fmt.Sprintf("- %s (%s)\n", ref.Title, ref.Source))
			}
		}
		b.WriteString("\n")
	}

	if hasIssues {
		b.WriteString("**Known Issues:**\n\n")
		for _, ki := range ragCtx.KnownIssues {
			fix := ""
			if ki.FixVersion != "" {
				fix = fmt.Sprintf(" (fix: %s)", ki.FixVersion)
			}
			b.WriteString(fmt.Sprintf("- **%s:** %s%s\n", ki.ID, ki.Summary, fix))
		}
		b.WriteString("\n")
	}
}

func extractClusterVersion(input ReportInput) string {
	if input.InfraReport == nil {
		return ""
	}
	for _, dim := range input.InfraReport.Dimensions {
		if dim.ID == healthcheck.DimClusterVersion && dim.Status == healthcheck.StatusPass {
			s := dim.Summary
			if idx := strings.Index(s, "Cluster version "); idx >= 0 {
				rest := s[idx+len("Cluster version "):]
				if sp := strings.IndexAny(rest, " ("); sp > 0 {
					return rest[:sp]
				}
				return rest
			}
		}
	}
	return ""
}

func failedDimensionsInCausalOrder(input ReportInput) []healthcheck.DimensionResult {
	order := []healthcheck.DimensionID{
		healthcheck.DimCatalogSource,
		healthcheck.DimSubscription,
		healthcheck.DimInstallPlan,
		healthcheck.DimOperatorGroup,
		healthcheck.DimCSVPhase,
		healthcheck.DimCSVRequirements,
		healthcheck.DimDeploymentReady,
		healthcheck.DimPodHealth,
		healthcheck.DimContainerRestarts,
		healthcheck.DimImagePull,
		healthcheck.DimScheduling,
		healthcheck.DimWarningEvents,
		healthcheck.DimCRDEstablished,
		healthcheck.DimManagedClusters,
		healthcheck.DimBackupRestore,
	}
	orderMap := make(map[healthcheck.DimensionID]int, len(order))
	for i, id := range order {
		orderMap[id] = i
	}

	var failed []healthcheck.DimensionResult
	if input.HealthReport != nil {
		for _, dim := range input.HealthReport.Dimensions {
			if dim.Status == healthcheck.StatusFail || dim.Status == healthcheck.StatusWarn {
				failed = append(failed, dim)
			}
		}
	}
	if input.InfraReport != nil {
		for _, dim := range input.InfraReport.Dimensions {
			if dim.Status == healthcheck.StatusFail || dim.Status == healthcheck.StatusWarn {
				failed = append(failed, dim)
			}
		}
	}

	sort.Slice(failed, func(i, j int) bool {
		oi, oki := orderMap[failed[i].ID]
		oj, okj := orderMap[failed[j].ID]
		if !oki {
			oi = 100
		}
		if !okj {
			oj = 100
		}
		if oi != oj {
			return oi < oj
		}
		if failed[i].Status != failed[j].Status {
			return failed[i].Status == healthcheck.StatusFail
		}
		return false
	})
	return failed
}

func buildTechnicalSummary(input ReportInput) string {
	op := input.OperatorState
	version := extractClusterVersion(input)
	var parts []string

	versionClause := ""
	if version != "" {
		versionClause = " on OCP " + version
	}

	switch {
	case op.Faulty && op.FailureReason != "":
		parts = append(parts, fmt.Sprintf("The **%s** operator failed%s.", op.PackageName, versionClause))

		for _, cond := range op.Conditions {
			if cond.Status != "True" || cond.Message == "" {
				continue
			}
			msg := cond.Message
			if len(msg) > 200 {
				msg = msg[:200] + "..."
			}
			stateDesc := op.State
			if stateDesc == "" {
				stateDesc = "unresolved"
			}
			parts = append(parts, fmt.Sprintf("The subscription entered a `%s` state with condition `%s`: %q.", stateDesc, cond.Type, msg))
			break
		}
		if len(parts) == 1 {
			parts = append(parts, fmt.Sprintf("Failure reason: %s.", op.FailureReason))
		}

	case op.Faulty:
		stateLabel := op.State
		if stateLabel == "" {
			stateLabel = "unknown"
		}
		parts = append(parts, fmt.Sprintf("The **%s** operator is in a faulty state (`%s`)%s.", op.PackageName, stateLabel, versionClause))

	case input.HealthReport != nil && input.HealthReport.Failed > 0:
		parts = append(parts, fmt.Sprintf("The **%s** operator subscription is healthy (`%s`) but %d health dimension(s) indicate runtime problems%s.",
			op.PackageName, op.State, input.HealthReport.Failed, versionClause))

		failed := failedDimensionsInCausalOrder(input)
		if len(failed) > 0 {
			primary := failed[0]
			if len(primary.Evidence) > 0 {
				ev := primary.Evidence[0]
				if len(ev) > 150 {
					ev = ev[:150] + "..."
				}
				parts = append(parts, fmt.Sprintf("The primary failure is in %s: %s (`%s`).", primary.Name, primary.Summary, ev))
			} else {
				parts = append(parts, fmt.Sprintf("The primary failure is in %s: %s.", primary.Name, primary.Summary))
			}
		}

	default:
		parts = append(parts, fmt.Sprintf("The **%s** operator passed all systematic health checks%s.", op.PackageName, versionClause))
	}

	if op.InstalledCSV == "" && (op.Faulty || (input.HealthReport != nil && input.HealthReport.Failed > 0)) {
		parts = append(parts, "No CSV was installed and no operator pods were deployed.")
	}

	if input.RAGContext != nil && input.RAGContext.IssueClassification != "" && input.RAGContext.IssueClassification != "unknown" {
		parts = append(parts, fmt.Sprintf("The root cause is classified as a **%s** issue.", input.RAGContext.IssueClassification))
	}

	return strings.Join(parts, " ")
}

func renderEvidenceBlock(b *strings.Builder, evidence []string, maxItems int) {
	if len(evidence) == 0 {
		return
	}
	b.WriteString("```\n")
	for i, e := range evidence {
		if i >= maxItems {
			b.WriteString(fmt.Sprintf("... and %d more\n", len(evidence)-maxItems))
			break
		}
		b.WriteString(e + "\n")
	}
	b.WriteString("```\n")
}

func determineSeverity(input ReportInput) string {
	for _, dims := range [][]healthcheck.DimensionResult{
		func() []healthcheck.DimensionResult {
			if input.HealthReport != nil {
				return input.HealthReport.Dimensions
			}
			return nil
		}(),
		func() []healthcheck.DimensionResult {
			if input.InfraReport != nil {
				return input.InfraReport.Dimensions
			}
			return nil
		}(),
	} {
		for _, dim := range dims {
			if dim.Severity == healthcheck.SeverityCritical {
				return "Critical"
			}
		}
	}
	if input.HealthReport != nil && input.HealthReport.Warnings > 0 {
		return "Warning"
	}
	return "Healthy"
}

func dimensionSignificance(dim healthcheck.DimensionResult, classification string) string {
	switch dim.ID {
	case healthcheck.DimCatalogSource:
		return "A CatalogSource failure prevents OLM from discovering any operators, blocking all installations and upgrades in this namespace."
	case healthcheck.DimSubscription:
		return "This subscription failure prevents CSV installation, which blocks all operator functionality. No controller pods can be deployed until the subscription resolves."
	case healthcheck.DimInstallPlan:
		return "An InstallPlan failure means the resolved operator version cannot be installed. The CSV and operator deployment are blocked."
	case healthcheck.DimCSVPhase:
		if classification == "configuration" {
			return "The CSV did not reach the Succeeded phase, indicating a configuration or dependency issue that prevents operator deployment."
		}
		return "The CSV did not reach the Succeeded phase. The operator controller cannot start until the CSV is successfully installed."
	case healthcheck.DimCSVRequirements:
		return "Unmet CSV requirements indicate missing API resources or CRDs that the operator depends on."
	case healthcheck.DimDeploymentReady:
		return "The operator deployment is not available, meaning the controller is not running and cannot reconcile custom resources."
	case healthcheck.DimPodHealth:
		return "Unhealthy pods indicate the operator controller is failing to start or crashing. This prevents all operator functionality."
	case healthcheck.DimContainerRestarts:
		return "Elevated container restarts suggest the operator is crash-looping, potentially due to a code bug, misconfiguration, or missing dependency."
	case healthcheck.DimImagePull:
		return "Image pull failures prevent the operator pod from starting. This may indicate a disconnected environment, registry authentication issue, or incorrect image reference."
	case healthcheck.DimScheduling:
		return "Pods cannot be scheduled, indicating insufficient cluster resources, node affinity conflicts, or taint/toleration mismatches."
	case healthcheck.DimNodeHealth:
		return "A NotReady node reduces cluster capacity and may cause workload disruptions across multiple operators."
	default:
		if dim.Status == healthcheck.StatusFail {
			return fmt.Sprintf("This %s check failure requires investigation to determine its impact on operator functionality.", dim.Category)
		}
		return fmt.Sprintf("This %s check warning should be monitored to prevent escalation.", dim.Category)
	}
}

func writeSimilarIssues(b *strings.Builder, input ReportInput) {
	if len(input.SimilarIssues) == 0 {
		return
	}

	b.WriteString("## Similar Issues from Past Sessions\n\n")
	b.WriteString("| Similarity | Operator | Classification | Resolution | Seen |\n")
	b.WriteString("|------------|----------|----------------|------------|------|\n")
	for i, si := range input.SimilarIssues {
		if i >= 10 {
			break
		}
		resolution := si.Resolution
		if resolution == "" {
			resolution = "-"
		}
		classification := si.Classification
		if classification == "" {
			classification = "-"
		}
		b.WriteString(fmt.Sprintf("| %.0f%% | %s | %s | %s | %dx |\n",
			si.Similarity*100, si.Operator, classification, truncate(resolution, 40), si.HitCount))
	}
	b.WriteString("\n")

	for _, si := range input.SimilarIssues {
		if si.Similarity == 1.0 && si.Resolution != "" {
			b.WriteString(fmt.Sprintf("> **Exact match found!** This issue was previously resolved: %s\n\n", si.Resolution))
			break
		}
	}
}

func writeLearningInsights(b *strings.Builder, input ReportInput) {
	if input.LearningInsights == nil {
		return
	}
	li := input.LearningInsights

	hasContent := len(li.FrameStats) > 0 || len(li.TopPatterns) > 0
	if !hasContent {
		return
	}

	b.WriteString("## Learning Insights\n\n")

	if len(li.FrameStats) > 0 {
		b.WriteString("### Frame Accuracy (historical)\n\n")
		b.WriteString("| Frame | Hypotheses | Confirmed | Accuracy | Traps |\n")
		b.WriteString("|-------|-----------|-----------|----------|-------|\n")
		for _, fs := range li.FrameStats {
			accuracy := 0.0
			if fs.Total > 0 {
				accuracy = float64(fs.Confirmed) / float64(fs.Total) * 100
			}
			b.WriteString(fmt.Sprintf("| %s | %d | %d | %.0f%% | %d |\n",
				fs.FrameID, fs.Total, fs.Confirmed, accuracy, fs.TrapCount))
		}
		b.WriteString("\n")
	}

	if len(li.TopPatterns) > 0 {
		b.WriteString("### Pattern Frequency\n\n")
		for _, p := range li.TopPatterns {
			b.WriteString(fmt.Sprintf("- **%s** — seen %dx (confidence: %.0f%%)\n",
				p.Pattern, p.Count, p.Confidence*100))
		}
		b.WriteString("\n")
	}
}

func writeAppendix(b *strings.Builder, input ReportInput) {
	b.WriteString("## Appendix\n\n")
	op := input.OperatorState
	b.WriteString("### Operator State\n\n")
	b.WriteString(fmt.Sprintf("- Installed CSV: `%s` (%s)\n", op.InstalledCSV, op.InstalledVersion))
	b.WriteString(fmt.Sprintf("- Current CSV: `%s`\n", op.CurrentCSV))
	b.WriteString(fmt.Sprintf("- Channel: `%s`\n", op.Channel))
	b.WriteString(fmt.Sprintf("- Subscription state: `%s`\n", op.State))

	if input.InstalledBundle != nil {
		ib := input.InstalledBundle
		b.WriteString(fmt.Sprintf("\n### Installed Bundle\n\n- Version: %s\n- Commit: %s\n- URL: %s\n",
			ib.Version, ib.Commit, ib.URL))
	}
	if input.TargetBundle != nil {
		tb := input.TargetBundle
		b.WriteString(fmt.Sprintf("\n### Target Bundle\n\n- Version: %s\n- Commit: %s\n- URL: %s\n",
			tb.Version, tb.Commit, tb.URL))
	}
	if input.CommitDelta != nil {
		cd := input.CommitDelta
		b.WriteString(fmt.Sprintf("\n### Code Delta\n\n- Files changed: %d\n- Additions: +%d\n- Deletions: -%d\n",
			len(cd.FilesChanged), cd.Additions, cd.Deletions))
	}
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "|", "/")
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
