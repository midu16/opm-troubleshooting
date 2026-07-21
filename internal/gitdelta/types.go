package gitdelta

// CommitDelta represents the difference between two git commits.
type CommitDelta struct {
	RepoURL      string
	OldCommit    string
	NewCommit    string
	DiffSummary  string   // Full git diff output
	FilesChanged []string // List of changed file paths
	Additions    int
	Deletions    int
}

// DeltaRequest specifies what commit range to analyze.
type DeltaRequest struct {
	RepoURL   string
	OldCommit string
	NewCommit string
}
