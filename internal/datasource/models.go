package datasource

import "time"

// NodeState represents the health state of a cluster node.
type NodeState struct {
	Name             string
	Ready            bool
	Unschedulable    bool
	Conditions       []Condition
	KubeletVersion   string
	OSImage          string
	ContainerRuntime string
	Roles            []string
	Capacity         ResourceList
	Allocatable      ResourceList
}

// ResourceList holds CPU/memory quantities as raw strings.
type ResourceList struct {
	CPU    string
	Memory string
	Pods   string
}

// Condition is a generic Kubernetes-style status condition.
type Condition struct {
	Type               string
	Status             string
	Reason             string
	Message            string
	LastTransitionTime time.Time
}

// OperatorState represents OLM subscription state from either data source.
type OperatorState struct {
	PackageName      string
	Namespace        string
	Channel          string
	InstalledCSV     string
	CurrentCSV       string
	InstalledVersion string
	State            string
	Conditions       []Condition
	InstallPlanRef   string
	RootCause        *RootCauseDetail
	Faulty           bool
	FailureReason    string
}

// RootCauseDetail contains missing dependency information from InstallPlan analysis.
type RootCauseDetail struct {
	MissingCRDs         []string
	MissingAPIs         []string
	UnknownResources    []string
	NotPresentResources []string
	PodErrors           []string
	RawFailureMessage   string
}

// CSVState represents a ClusterServiceVersion snapshot.
type CSVState struct {
	Name       string
	Namespace  string
	Phase      string
	Reason     string
	Message    string
	Version    string
	Conditions []Condition
}

// InstallPlanState represents an OLM InstallPlan snapshot.
type InstallPlanState struct {
	Name       string
	Namespace  string
	Phase      string
	Approved   bool
	Failed     bool
	Conditions []Condition
	Steps      []InstallPlanStep
}

// InstallPlanStep describes a single resource in an InstallPlan.
type InstallPlanStep struct {
	Group     string
	Kind      string
	Name      string
	Version   string
	Status    string
	Resolving string
}

// DeploymentState represents a Deployment resource snapshot.
type DeploymentState struct {
	Name           string
	Namespace      string
	Replicas       int32
	ReadyReplicas  int32
	Available      bool
	Progressing    bool
	ProgressingMsg string
	UnavailableMsg string
}

// PodState represents a Pod resource snapshot.
type PodState struct {
	Name             string
	Namespace        string
	Phase            string
	Ready            bool
	RestartCount     int32
	WaitingReason    string
	WaitingMessage   string
	TerminatedReason string
}

// EventState represents a Kubernetes event.
type EventState struct {
	Type    string
	Reason  string
	Message string
	Object  string
}

// IDMSState represents an ImageDigestMirrorSet resource.
type IDMSState struct {
	Name    string
	Mirrors []MirrorEntry
}

// MirrorEntry describes a source-to-mirror mapping.
type MirrorEntry struct {
	Source  string
	Mirrors []string
}

// EtcdMemberState represents an etcd cluster member's health.
type EtcdMemberState struct {
	Name      string
	ID        string
	PeerURL   string
	ClientURL string
	IsLeader  bool
	IsHealthy bool
	DBSize    int64
}

// ClusterOperatorState represents an OpenShift ClusterOperator status.
type ClusterOperatorState struct {
	Name        string
	Available   bool
	Progressing bool
	Degraded    bool
	Version     string
	Conditions  []Condition
}

// MCPState represents a MachineConfigPool status.
type MCPState struct {
	Name                 string
	MachineCount         int32
	ReadyMachineCount    int32
	UpdatedMachineCount  int32
	DegradedMachineCount int32
	Paused               bool
	Updating             bool
	Degraded             bool
	Conditions           []Condition
}

// RouteState represents an OpenShift Route.
type RouteState struct {
	Name      string
	Namespace string
	Host      string
	Admitted  bool
	TLS       bool
}

// CertificateState represents a TLS certificate found in the cluster.
type CertificateState struct {
	Name      string
	Namespace string
	Subject   string
	Issuer    string
	NotBefore time.Time
	NotAfter  time.Time
	IsCA      bool
	Source    string // "secret", "configmap", or "apiserver"
}

// PVState represents a PersistentVolume snapshot.
type PVState struct {
	Name          string
	Capacity      string
	Phase         string // Bound, Available, Released, Failed, Pending
	StorageClass  string
	ReclaimPolicy string
	ClaimRef      string
}

// PVCState represents a PersistentVolumeClaim snapshot.
type PVCState struct {
	Name         string
	Namespace    string
	Phase        string // Bound, Pending, Lost
	VolumeName   string
	StorageClass string
	Capacity     string
}

// StorageClassState represents a StorageClass resource.
type StorageClassState struct {
	Name        string
	Provisioner string
	IsDefault   bool
	Parameters  map[string]string
}

// NetworkConfigState represents the cluster network configuration.
type NetworkConfigState struct {
	NetworkType    string // OVNKubernetes, OpenShiftSDN
	ClusterNetwork []NetworkRange
	ServiceNetwork []string
	Conditions     []Condition
}

// NetworkRange describes a CIDR block and host prefix.
type NetworkRange struct {
	CIDR       string
	HostPrefix int32
}

// ClusterVersionState represents the cluster version and upgrade status.
type ClusterVersionState struct {
	Version     string
	Channel     string
	ClusterID   string
	Available   bool
	Progressing bool
	Failing     bool
	Conditions  []Condition
	History     []UpdateHistory
}

// UpdateHistory records a cluster version update event.
type UpdateHistory struct {
	Version     string
	State       string // Completed, Partial
	StartedAt   time.Time
	CompletedAt time.Time
}

// APIServerState represents the API server health.
type APIServerState struct {
	Available  bool
	Conditions []Condition
}

// CatalogSourceState represents an OLM CatalogSource.
type CatalogSourceState struct {
	Name       string
	Namespace  string
	Image      string
	Status     string
	Conditions []Condition
}

// OperatorGroupState represents an OLM OperatorGroup.
type OperatorGroupState struct {
	Name             string
	Namespace        string
	TargetNamespaces []string
}
