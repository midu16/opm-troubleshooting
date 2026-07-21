package rag

type Collection string

const (
	CollDocs        Collection = "ocp_docs"
	CollCode        Collection = "operator_code"
	CollTelco       Collection = "telco_configs"
	CollKnownIssues Collection = "known_issues"
	CollManifests   Collection = "manifests"
)

var AllCollections = []Collection{CollDocs, CollCode, CollTelco, CollKnownIssues, CollManifests}

type Document struct {
	ID       string
	Content  string
	Metadata map[string]string
}

type TroubleshootResult struct {
	Summary           string         `json:"summary"`
	DocumentationRefs []DocReference `json:"documentation_refs"`
	KnownIssues       []KnownIssue   `json:"known_issues"`
	ConfigAdvice      []ConfigAdvice `json:"config_advice"`
	Confidence        float64        `json:"confidence"`
}

type DocReference struct {
	Title   string `json:"title"`
	Source  string `json:"source"`
	Excerpt string `json:"excerpt"`
	URL     string `json:"url"`
}

type KnownIssue struct {
	ID         string `json:"id"`
	Summary    string `json:"summary"`
	Workaround string `json:"workaround"`
	FixVersion string `json:"fix_version"`
}

type ConfigAdvice struct {
	Component string `json:"component"`
	Reference string `json:"reference"`
	Advice    string `json:"advice"`
}

type SearchResult struct {
	Documents []DocReference `json:"documents"`
	Query     string         `json:"query"`
}

type FreshnessStatus struct {
	Fresh         bool   `json:"fresh"`
	IngestedAt    string `json:"ingested_at"`
	IngestCommits map[string]string `json:"ingest_commits"`
	Message       string `json:"message"`
}
