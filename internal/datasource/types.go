package datasource

// ClusterDataSource is the common interface for reading cluster state from
// either a must-gather dump or a live cluster via kubeconfig.
type ClusterDataSource interface {
	// OLM resources
	GetSubscriptions(ns string) ([]OperatorState, error)
	GetCSVs(ns string) ([]CSVState, error)
	GetInstallPlans(ns string) ([]InstallPlanState, error)
	GetCatalogSources(ns string) ([]CatalogSourceState, error)
	GetOperatorGroups(ns string) ([]OperatorGroupState, error)

	// Workload resources
	GetDeployments(ns string) ([]DeploymentState, error)
	GetPods(ns string) ([]PodState, error)
	GetEvents(ns string) ([]EventState, error)

	// Cluster-scoped resources
	GetNodes() ([]NodeState, error)
	GetNamespaces() ([]string, error)
	GetClusterOperators() ([]ClusterOperatorState, error)
	GetClusterVersion() (*ClusterVersionState, error)
	GetMachineConfigPools() ([]MCPState, error)

	// Networking
	GetNetworkConfig() (*NetworkConfigState, error)
	GetRoutes(ns string) ([]RouteState, error)

	// Storage
	GetPVs() ([]PVState, error)
	GetPVCs(ns string) ([]PVCState, error)
	GetStorageClasses() ([]StorageClassState, error)

	// Security
	GetCertificates() ([]CertificateState, error)
	GetIDMS() ([]IDMSState, error)

	// Infrastructure
	GetEtcdMembers() ([]EtcdMemberState, error)
	GetAPIServerStatus() (*APIServerState, error)

	// Metadata
	SourceType() string // "must-gather" or "live-cluster"
}
