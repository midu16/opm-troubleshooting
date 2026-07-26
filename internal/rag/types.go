package rag

type Collection string

const (
	CollDocs        Collection = "ocp_docs"
	CollCode        Collection = "operator_code"
	CollTelco       Collection = "telco_configs"
	CollKnownIssues Collection = "known_issues"
	CollManifests   Collection = "manifests"
	CollACMDocs     Collection = "acm_docs"
)

var AllCollections = []Collection{CollDocs, CollCode, CollTelco, CollKnownIssues, CollManifests, CollACMDocs}

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

	SymptomAnalysis        []SymptomEvidence  `json:"symptom_analysis,omitempty"`
	IssueClassification    string             `json:"issue_classification,omitempty"`
	ClassificationEvidence []string           `json:"classification_evidence,omitempty"`
	RemediationSteps       []RemediationStep  `json:"remediation_steps,omitempty"`
	RelevantCodePaths      []CodePathEvidence `json:"relevant_code_paths,omitempty"`
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

// DeepTroubleshootInput carries rich symptom data for multi-phase deep analysis.
type DeepTroubleshootInput struct {
	Operator          string
	Namespace         string
	OCPVersion        string
	FailedDimensions  []DimensionSymptom
	UnhealthyPods     []PodSymptom
	UnavailableDeploy []DeploymentSymptom
	WarningEvents     []EventSymptom
	FailureReason     string
	CurrentCSV        string
	InstalledCSV      string
	Channel           string
	SubscriptionState string
	InfraFailures     []DimensionSymptom
}

type DimensionSymptom struct {
	DimensionID string
	Name        string
	Category    string
	Status      string
	Summary     string
	Evidence    []string
}

type PodSymptom struct {
	Name             string
	Phase            string
	WaitingReason    string
	WaitingMessage   string
	TerminatedReason string
	RestartCount     int32
}

type DeploymentSymptom struct {
	Name           string
	Replicas       int32
	ReadyReplicas  int32
	ProgressingMsg string
	UnavailableMsg string
}

type EventSymptom struct {
	Object  string
	Reason  string
	Message string
}

type SymptomEvidence struct {
	Symptom       string         `json:"symptom"`
	DimensionID   string         `json:"dimension_id"`
	Query         string         `json:"query"`
	DocMatches    []DocReference `json:"doc_matches,omitempty"`
	CodeMatches   []DocReference `json:"code_matches,omitempty"`
	ConfigMatches []ConfigAdvice `json:"config_matches,omitempty"`
	KnownIssues   []KnownIssue   `json:"known_issues,omitempty"`
	Relevance     float64        `json:"relevance"`
}

type RemediationStep struct {
	Step       int     `json:"step"`
	Priority   string  `json:"priority"`
	Action     string  `json:"action"`
	Source     string  `json:"source"`
	Confidence float64 `json:"confidence"`
}

type CodePathEvidence struct {
	Declaration string `json:"declaration"`
	FilePath    string `json:"file_path"`
	Repo        string `json:"repo"`
	RepoURL     string `json:"repo_url"`
	Excerpt     string `json:"excerpt"`
	Relevance   string `json:"relevance"`
}

type FreshnessStatus struct {
	Fresh         bool              `json:"fresh"`
	IngestedAt    string            `json:"ingested_at"`
	IngestCommits map[string]string `json:"ingest_commits"`
	Message       string            `json:"message"`
}
