package analysis

import (
	"github.com/midu16/opm-troubleshooting/internal/adhd"
	"github.com/midu16/opm-troubleshooting/internal/claudeapi"
	"github.com/midu16/opm-troubleshooting/internal/codeanalysis"
	"github.com/midu16/opm-troubleshooting/internal/gitdelta"
	"github.com/midu16/opm-troubleshooting/internal/healthcheck"
	"github.com/midu16/opm-troubleshooting/internal/imageinspect"
	"github.com/midu16/opm-troubleshooting/internal/mustgather"
	"github.com/midu16/opm-troubleshooting/internal/noise"
	"github.com/midu16/opm-troubleshooting/internal/openshift"
	"github.com/midu16/opm-troubleshooting/internal/rca"
	"github.com/midu16/opm-troubleshooting/internal/session"
	"github.com/midu16/opm-troubleshooting/internal/telco"
)

// AnalysisConfig specifies input parameters for must-gather analysis.
type AnalysisConfig struct {
	MustGatherPath string
	CatalogRef     string // Optional: catalog to resolve target versions from
	TargetVersion  string // Optional: specific version to compare against
	PackageName    string // Optional: analyze only this package (empty = all faulty)

	// Telco production options
	TelcoSuite    bool              // Analyze full OADP/TALM/IDMS/MCH suite
	HealthCheck   bool              // Run 20-dimension health checks (default true in must-gather mode)
	Environment   noise.Environment // lab, disconnected, kvm, production
	SourceRepo    string            // Local operator source repo for code analysis
	GenerateRCA   bool              // Generate professional RCA markdown
	ClusterName   string            // Cluster identifier for session persistence
	StateDir      string            // Session store directory
	Session       *session.Record   // Existing session for RCA context

	// ADHD divergent analysis
	ADHDEnabled bool   // Enable ADHD multi-frame analysis
	ADHDFrames  int    // Number of divergence frames
	ADHDDepth   string // "quick", "standard", "deep"

	// Metadata, learning, and repo correlation
	MetadataDir           string
	EnableLearning        bool
	EnableRepoCorrelation bool
	GitHubToken           string
}

// FaultReport contains complete analysis for a single faulty operator.
type FaultReport struct {
	Operator        mustgather.OperatorState
	TelcoProfile    *telco.Profile
	InstalledBundle *imageinspect.BundleInfo
	TargetBundle    *imageinspect.BundleInfo
	CommitDelta     *gitdelta.CommitDelta
	ClaudeAnalysis  *claudeapi.AnalysisResponse
	RCAPatterns     []rca.PatternMatch
	Recommendations []rca.AnalysisRecommendation
	HealthReport    *healthcheck.Report
	InfraReport     *healthcheck.Report
	NoiseReport     *noise.FilterReport
	CodeAnalysis    *codeanalysis.Result
	ADHDResult      *adhd.DiagnosisResult
	RepoCorrelation  *openshift.Correlation
	SimilarIssues    []rca.SimilarIssueData
	LearningInsights *rca.LearningInsightsData
	RCADocument     *rca.Document
	Errors          []error
}

// AnalysisResult holds the complete output of must-gather analysis.
type AnalysisResult struct {
	MustGatherPath string
	Environment    noise.Environment
	ClusterName    string
	Session        *session.Record
	Operators      []mustgather.OperatorState
	FaultyReports  []FaultReport
	RCADocuments   []rca.Document
	TotalOperators int
	FaultyCount    int
	AnalyzedCount  int
	Errors         []error
}
