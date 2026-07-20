package mustgather

import "time"

// OperatorState represents the runtime state of an operator from must-gather.
type OperatorState struct {
	PackageName      string
	Namespace        string
	Channel          string
	InstalledCSV     string // status.installedCSV from Subscription
	CurrentCSV       string // status.currentCSV from Subscription
	InstalledVersion string // Extracted from CSV name or CSV manifest
	State            string // Subscription state: AtLatestKnown, UpgradePending, Failed
	Conditions       []Condition
	InstallPlanRef   string            // Name of referenced InstallPlan from subscription
	RootCause        *RootCauseDetail  // Detailed root cause from InstallPlan analysis
	Faulty           bool              // Computed: true if state != AtLatestKnown or unhealthy conditions
	FailureReason    string            // Human-readable summary of failure
}

// Condition represents a Kubernetes condition from Subscription or CSV status.
type Condition struct {
	Type               string
	Status             string
	Reason             string
	Message            string
	LastTransitionTime time.Time
}

// Removed - using detailed InstallPlan struct from installplan.go instead

// ParseResult holds all operators found in must-gather.
type ParseResult struct {
	Operators   []OperatorState
	FaultyCount int
	ParseErrors []error
}
