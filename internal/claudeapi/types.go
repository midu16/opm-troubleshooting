package claudeapi

// AnalysisRequest encapsulates the fault analysis prompt.
type AnalysisRequest struct {
	OperatorName     string
	FailureSymptoms  string // Human-readable description of failure
	InstalledVersion string
	TargetVersion    string
	CommitDelta      string // Git diff output
	FilesChanged     []string
}

// AnalysisResponse holds Claude's fault isolation analysis.
type AnalysisResponse struct {
	Summary            string   // High-level summary
	LikelyCauses       []string // Specific code changes that may have caused failure
	RecommendedActions []string // Suggested troubleshooting steps
	Confidence         string   // Low/Medium/High
	RawResponse        string   // Full Claude response for debugging
}
