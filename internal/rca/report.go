package rca

import (
	"fmt"
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
	ClusterName    string
	Environment    noise.Environment
	MustGatherPath string
	Operator       string
	Namespace      string
	OperatorState  mustgather.OperatorState
	InstalledBundle *imageinspect.BundleInfo
	TargetBundle    *imageinspect.BundleInfo
	CommitDelta     *gitdelta.CommitDelta
	ClaudeAnalysis  *claudeapi.AnalysisResponse
	RCAPatterns     []PatternMatch
	Recommendations []AnalysisRecommendation
	HealthReport    *healthcheck.Report
	InfraReport     *healthcheck.Report
	NoiseReport     *noise.FilterReport
	CodeAnalysis    *codeanalysis.Result
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
	writeHealthDimensions(&b, input)
	writeInfraHealthDimensions(&b, input)
	writeNoiseAnalysis(&b, input)
	writeRootCause(&b, input)
	writeADHDAnalysis(&b, input)
	writeSourceCodeCorrelation(&b, input)
	writeRAGContext(&b, input)
	writeSimilarIssues(&b, input)
	writeLearningInsights(&b, input)
	writeCodeEvidence(&b, input)
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

	realIssues := 0
	cosmetic := 0
	if input.NoiseReport != nil {
		realIssues = input.NoiseReport.RealIssues
		cosmetic = input.NoiseReport.CosmeticAlerts
	}

	op := input.OperatorState
	if op.Faulty {
		b.WriteString(fmt.Sprintf(
			"Operator **%s** is in a faulty state (`%s`). ",
			op.PackageName, op.State,
		))
	} else if input.HealthReport != nil && input.HealthReport.Failed > 0 {
		b.WriteString(fmt.Sprintf(
			"Operator **%s** subscription appears healthy but **%d** health dimension(s) failed. ",
			op.PackageName, input.HealthReport.Failed,
		))
	} else {
		b.WriteString(fmt.Sprintf("Operator **%s** passed systematic health checks. ", op.PackageName))
	}

	if realIssues > 0 {
		b.WriteString(fmt.Sprintf("**%d real issue(s)** require action. ", realIssues))
	}
	if cosmetic > 0 {
		b.WriteString(fmt.Sprintf("**%d alert(s)** classified as cosmetic noise for `%s` environment. ", cosmetic, input.Environment))
	}
	b.WriteString("\n\n")
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

func writeCodeEvidence(b *strings.Builder, input ReportInput) {
	if input.CodeAnalysis == nil || len(input.CodeAnalysis.Matches) == 0 {
		return
	}
	ca := input.CodeAnalysis
	b.WriteString("## Code-Level Evidence\n\n")
	b.WriteString(fmt.Sprintf("%s\n\n", ca.Summary))
	b.WriteString("| File | Line | Pattern | Content |\n")
	b.WriteString("|------|------|---------|----------|\n")
	for i, m := range ca.Matches {
		if i >= 15 {
			b.WriteString(fmt.Sprintf("| ... | | | (%d more matches) |\n", len(ca.Matches)-15))
			break
		}
		b.WriteString(fmt.Sprintf(
			"| `%s` | %d | %s | %s |\n",
			m.FilePath, m.LineNumber, truncate(m.Pattern, 30), truncate(m.LineContent, 60),
		))
	}
	b.WriteString("\n")
}

func writeRecommendations(b *strings.Builder, input ReportInput) {
	b.WriteString("## Recommended Actions\n\n")
	priority := 1

	if input.ClaudeAnalysis != nil {
		for _, action := range input.ClaudeAnalysis.RecommendedActions {
			b.WriteString(fmt.Sprintf("%d. %s\n", priority, action))
			priority++
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
		b.WriteString(fmt.Sprintf("%d. **[ %s ]** %s — %s\n", priority, level, rec.Title, rec.Description))
		priority++
	}

	if input.HealthReport != nil {
		for _, dim := range input.HealthReport.Dimensions {
			if dim.Status == healthcheck.StatusFail && dim.Recommendation != "" {
				b.WriteString(fmt.Sprintf("%d. **[%s]** %s\n", priority, dim.Name, dim.Recommendation))
				priority++
			}
		}
	}

	if priority == 1 {
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
	if rc.CommitPinned && rc.BundleCommit != "" {
		commitShort := rc.BundleCommit
		if len(commitShort) > 12 {
			commitShort = commitShort[:12]
		}
		b.WriteString(fmt.Sprintf("Analysis pinned to deployed commit `%s`. ", commitShort))
		b.WriteString("This ensures code correlation reflects the exact version running in the cluster, not the latest upstream changes.\n\n")
	} else if rc.BundleCommit != "" {
		commitShort := rc.BundleCommit
		if len(commitShort) > 12 {
			commitShort = commitShort[:12]
		}
		b.WriteString(fmt.Sprintf("Deployed commit: `%s` (analysis performed at HEAD — commit-pinned analysis was unavailable).\n\n", commitShort))
	} else {
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

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "|", "/")
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
