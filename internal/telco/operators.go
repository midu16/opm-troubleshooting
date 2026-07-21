package telco

// Category groups telco production operators by functional domain.
type Category string

const (
	CategoryClusterMgmt Category = "cluster-management"
	CategoryLifecycle   Category = "lifecycle"
	CategoryLogging     Category = "logging"
	CategoryNetworking  Category = "networking"
	CategoryStorage     Category = "storage"
	CategoryODF         Category = "odf"
	CategoryBackup      Category = "backup"
	CategoryConfig      Category = "config"
	CategoryGitOps      Category = "gitops"
	CategorySecurity    Category = "security"
)

// OperatorID identifies a telco production operator profile.
type OperatorID string

const (
	OperatorOADP OperatorID = "OADP"
	OperatorTALM OperatorID = "TALM"
	OperatorIDMS OperatorID = "IDMS"
	OperatorMCH  OperatorID = "MCH"
	OperatorMCE  OperatorID = "MCE"
	OperatorACM  OperatorID = "ACM"

	OperatorClusterLogging      OperatorID = "CLUSTER_LOGGING"
	OperatorNMState             OperatorID = "NMSTATE"
	OperatorLifecycleAgent      OperatorID = "LIFECYCLE_AGENT"
	OperatorLocalStorage        OperatorID = "LOCAL_STORAGE"
	OperatorLVMS                OperatorID = "LVMS"
	OperatorMetalLB             OperatorID = "METALLB"
	OperatorNUMAResources       OperatorID = "NUMARESOURCES"
	OperatorOCloudManager       OperatorID = "O_CLOUD_MANAGER"
	OperatorCephCSI             OperatorID = "CEPHCSI"
	OperatorMCG                 OperatorID = "MCG"
	OperatorOCSClient             OperatorID = "OCS_CLIENT"
	OperatorOCS                   OperatorID = "OCS"
	OperatorODFCSIAddons        OperatorID = "ODF_CSI_ADDONS"
	OperatorODFDependencies     OperatorID = "ODF_DEPENDENCIES"
	OperatorODFExternalSnapshot OperatorID = "ODF_EXTERNAL_SNAPSHOT"
	OperatorODF                 OperatorID = "ODF"
	OperatorODFPrometheus       OperatorID = "ODF_PROMETHEUS"
	OperatorRecipe              OperatorID = "RECIPE"
	OperatorRookCeph            OperatorID = "ROOK_CEPH"
	OperatorCertManager         OperatorID = "CERT_MANAGER"
	OperatorGitOps              OperatorID = "GITOPS"
	OperatorPTP                 OperatorID = "PTP"
	OperatorSRIOV               OperatorID = "SRIOV"
)

// Profile describes how to locate and diagnose a telco production operator.
type Profile struct {
	ID              OperatorID
	Category        Category
	PackageName     string
	DisplayName     string
	DefaultNS       string
	AltNamespaces   []string
	DeploymentNames []string
	LogPatterns     []string
	CRKinds         []string
}

// CoreSuite returns the original fast-diagnosis quartet (OADP, TALM, IDMS, MCH).
func CoreSuite() []Profile {
	return []Profile{
		OADP(),
		TALM(),
		IDMS(),
		MCH(),
	}
}

// Suite returns the full telco production operator suite for systematic redeployment checks.
func Suite() []Profile {
	return allProfiles
}

// AllProfiles returns every registered telco production profile.
func AllProfiles() []Profile {
	out := make([]Profile, len(allProfiles))
	copy(out, allProfiles)
	return out
}

// ProfilesByCategory returns profiles matching the given category.
func ProfilesByCategory(cat Category) []Profile {
	out := make([]Profile, 0)
	for _, p := range allProfiles {
		if p.Category == cat {
			out = append(out, p)
		}
	}
	return out
}

// ProfileByPackage returns the telco profile for an OLM package name, if any.
func ProfileByPackage(packageName string) (Profile, bool) {
	p, ok := packageIndex[packageName]
	return p, ok
}

// ProfileByID returns a profile by operator ID.
func ProfileByID(id OperatorID) (Profile, bool) {
	if id == OperatorMCH {
		return MCH(), true
	}
	for _, p := range allProfiles {
		if p.ID == id {
			return p, true
		}
	}
	return Profile{}, false
}

// PackageNames returns OLM package names in the production suite (excludes IDMS).
func PackageNames() []string {
	names := make([]string, 0, len(allProfiles))
	for _, p := range allProfiles {
		if p.PackageName != "" {
			names = append(names, p.PackageName)
		}
	}
	return names
}

// ODFPackageNames returns ODF-related OLM package names.
func ODFPackageNames() []string {
	return packageNamesForCategory(CategoryODF)
}

func packageNamesForCategory(cat Category) []string {
	names := make([]string, 0)
	for _, p := range allProfiles {
		if p.Category == cat && p.PackageName != "" {
			names = append(names, p.PackageName)
		}
	}
	return names
}

// allProfiles is the canonical ordered production suite.
var allProfiles = []Profile{
	// Cluster management
	ACM(),
	MCE(),

	// Lifecycle & GitOps
	TALM(),
	LifecycleAgent(),
	GitOps(),

	// Logging & observability
	ClusterLogging(),

	// Networking
	NMState(),
	MetalLB(),
	SRIOV(),
	PTP(),
	NUMAResources(),

	// Platform config
	IDMS(),
	OCloudManager(),
	CertManager(),

	// Local / block storage
	LocalStorage(),
	LVMS(),

	// ODF stack
	ODF(),
	ODFDependencies(),
	RookCeph(),
	CephCSI(),
	OCS(),
	OCSClient(),
	MCG(),
	ODFCSIAddons(),
	ODFExternalSnapshot(),
	ODFPrometheus(),
	Recipe(),

	// Backup
	OADP(),
}

var packageIndex map[string]Profile

func init() {
	packageIndex = make(map[string]Profile, len(allProfiles))
	for _, p := range allProfiles {
		if p.PackageName != "" {
			packageIndex[p.PackageName] = p
		}
	}
}

// --- Cluster management ---

func ACM() Profile {
	return Profile{
		ID:              OperatorACM,
		Category:        CategoryClusterMgmt,
		PackageName:     "advanced-cluster-management",
		DisplayName:     "Advanced Cluster Management (ACM)",
		DefaultNS:       "open-cluster-management",
		AltNamespaces:   []string{"open-cluster-management-hub"},
		DeploymentNames: []string{"multiclusterhub-operator", "cluster-manager"},
		LogPatterns:     []string{"MultiClusterHub", "managed cluster", "placement", "policy", "Reconciler error"},
		CRKinds:         []string{"MultiClusterHub", "ManagedCluster", "Placement", "Policy"},
	}
}

// MCH is an alias for ACM (legacy fast-diagnosis ID).
func MCH() Profile {
	p := ACM()
	p.ID = OperatorMCH
	return p
}

func MCE() Profile {
	return Profile{
		ID:              OperatorMCE,
		Category:        CategoryClusterMgmt,
		PackageName:     "multicluster-engine",
		DisplayName:     "Multicluster Engine",
		DefaultNS:       "multicluster-engine",
		DeploymentNames: []string{"multicluster-engine-operator", "backplane-operator"},
		LogPatterns:     []string{"MultiClusterEngine", "AddOnPlacementScore", "managed cluster", "Reconciler error"},
		CRKinds:         []string{"MultiClusterEngine", "ClusterCurator", "ManagedClusterAddon"},
	}
}

// --- Lifecycle ---

func TALM() Profile {
	return Profile{
		ID:              OperatorTALM,
		Category:        CategoryLifecycle,
		PackageName:     "topology-aware-lifecycle-manager",
		DisplayName:     "Topology Aware Lifecycle Manager",
		DefaultNS:       "openshift-operators",
		DeploymentNames: []string{"cluster-group-upgrades-controller-manager"},
		LogPatterns:     []string{"ClusterGroupUpgrade", "remediationAction", "policy compliance", "managed cluster"},
		CRKinds:         []string{"ClusterGroupUpgrade", "PreCachingConfig"},
	}
}

func LifecycleAgent() Profile {
	return Profile{
		ID:              OperatorLifecycleAgent,
		Category:        CategoryLifecycle,
		PackageName:     "lifecycle-agent",
		DisplayName:     "Lifecycle Agent",
		DefaultNS:       "openshift-lifecycle-agent",
		LogPatterns:     []string{"lifecycle-agent", "image-based upgrade", "reboot", "Reconciler error"},
		CRKinds:         []string{"ImageBasedUpgrade", "LCAUpgrade"},
	}
}

func GitOps() Profile {
	return Profile{
		ID:              OperatorGitOps,
		Category:        CategoryGitOps,
		PackageName:     "openshift-gitops-operator",
		DisplayName:     "OpenShift GitOps",
		DefaultNS:       "openshift-operators",
		AltNamespaces:   []string{"openshift-gitops"},
		LogPatterns:     []string{"ArgoCD", "Application", "sync failed", "Reconciler error"},
		CRKinds:         []string{"ArgoCD", "Application", "AppProject"},
	}
}

// --- Logging ---

func ClusterLogging() Profile {
	return Profile{
		ID:              OperatorClusterLogging,
		Category:        CategoryLogging,
		PackageName:     "cluster-logging",
		DisplayName:     "Cluster Logging",
		DefaultNS:       "openshift-logging",
		DeploymentNames: []string{"cluster-logging-operator"},
		LogPatterns:     []string{"ClusterLogForwarder", "collector", "elasticsearch", "lokistack", "Reconciler error"},
		CRKinds:         []string{"ClusterLogForwarder", "LogFileMetricExporter", "Logging"},
	}
}

// --- Networking ---

func NMState() Profile {
	return Profile{
		ID:              OperatorNMState,
		Category:        CategoryNetworking,
		PackageName:     "kubernetes-nmstate-operator",
		DisplayName:     "Kubernetes NMState Operator",
		DefaultNS:       "openshift-nmstate",
		LogPatterns:     []string{"NodeNetworkConfigurationPolicy", "nmstate", "Reconciler error"},
		CRKinds:         []string{"NodeNetworkConfigurationPolicy", "NMState"},
	}
}

func MetalLB() Profile {
	return Profile{
		ID:              OperatorMetalLB,
		Category:        CategoryNetworking,
		PackageName:     "metallb-operator",
		DisplayName:     "MetalLB Operator",
		DefaultNS:       "openshift-metallb",
		LogPatterns:     []string{"MetalLB", "BGP", "L2Advertisement", "IPAddressPool", "Reconciler error"},
		CRKinds:         []string{"MetalLB", "BGPAdvertisement", "IPAddressPool", "L2Advertisement"},
	}
}

func SRIOV() Profile {
	return Profile{
		ID:              OperatorSRIOV,
		Category:        CategoryNetworking,
		PackageName:     "sriov-network-operator",
		DisplayName:     "SR-IOV Network Operator",
		DefaultNS:       "openshift-sriov-network-operator",
		LogPatterns:     []string{"SriovNetworkNodePolicy", "SriovNetwork", "VF", "Reconciler error"},
		CRKinds:         []string{"SriovNetworkNodePolicy", "SriovNetwork", "SriovOperatorConfig"},
	}
}

func PTP() Profile {
	return Profile{
		ID:              OperatorPTP,
		Category:        CategoryNetworking,
		PackageName:     "ptp-operator",
		DisplayName:     "PTP Operator",
		DefaultNS:       "openshift-ptp",
		LogPatterns:     []string{"PtpConfig", "linuxptp", "phc", "clock sync", "Reconciler error"},
		CRKinds:         []string{"PtpConfig", "PtpOperatorConfig"},
	}
}

func NUMAResources() Profile {
	return Profile{
		ID:              OperatorNUMAResources,
		Category:        CategoryNetworking,
		PackageName:     "numaresources-operator",
		DisplayName:     "NUMA Resources Operator",
		DefaultNS:       "openshift-numaresources",
		LogPatterns:     []string{"NUMAResourcesOperator", "NUMAResourcesScheduler", "topology", "Reconciler error"},
		CRKinds:         []string{"NUMAResourcesOperator", "NUMAResourcesScheduler", "TopologyAwareResource"},
	}
}

// --- Platform config ---

func IDMS() Profile {
	return Profile{
		ID:          OperatorIDMS,
		Category:    CategoryConfig,
		PackageName: "",
		DisplayName: "Image Digest Mirror Set",
		DefaultNS:   "openshift-config",
		LogPatterns: []string{"ImagePullBackOff", "mirror", "registry.redhat.io", "unauthorized", "x509"},
		CRKinds:     []string{"ImageDigestMirrorSet", "ImageTagMirrorSet", "ImageContentSourcePolicy"},
	}
}

func OCloudManager() Profile {
	return Profile{
		ID:              OperatorOCloudManager,
		Category:        CategoryConfig,
		PackageName:     "o-cloud-manager",
		DisplayName:     "O-Cloud Manager",
		DefaultNS:       "openshift-o-cloud-manager",
		LogPatterns:     []string{"o-cloud-manager", "Reconciler error"},
		CRKinds:         []string{"OCloudManager"},
	}
}

func CertManager() Profile {
	return Profile{
		ID:              OperatorCertManager,
		Category:        CategorySecurity,
		PackageName:     "openshift-cert-manager-operator",
		DisplayName:     "OpenShift Cert Manager Operator",
		DefaultNS:       "openshift-cert-manager-operator",
		AltNamespaces:   []string{"cert-manager"},
		LogPatterns:     []string{"cert-manager", "Certificate", "Issuer", "ACME", "Reconciler error"},
		CRKinds:         []string{"CertManager", "Certificate", "ClusterIssuer", "Issuer"},
	}
}

// --- Storage ---

func LocalStorage() Profile {
	return Profile{
		ID:              OperatorLocalStorage,
		Category:        CategoryStorage,
		PackageName:     "local-storage-operator",
		DisplayName:     "Local Storage Operator",
		DefaultNS:       "openshift-local-storage",
		LogPatterns:     []string{"LocalVolume", "LocalVolumeDiscovery", "discover", "Reconciler error"},
		CRKinds:         []string{"LocalVolume", "LocalVolumeDiscovery", "LocalVolumeSet"},
	}
}

func LVMS() Profile {
	return Profile{
		ID:              OperatorLVMS,
		Category:        CategoryStorage,
		PackageName:     "lvms-operator",
		DisplayName:     "LVM Storage Operator",
		DefaultNS:       "openshift-storage",
		AltNamespaces:   []string{"openshift-lvm-storage"},
		LogPatterns:     []string{"LVMCluster", "LogicalVolume", "thin pool", "Reconciler error"},
		CRKinds:         []string{"LVMCluster", "LVMVolumeGroup", "LogicalVolume"},
	}
}

// --- ODF stack ---

func ODF() Profile {
	return odfProfile(OperatorODF, "odf-operator", "OpenShift Data Foundation", []string{"StorageCluster"})
}

func ODFDependencies() Profile {
	return odfProfile(OperatorODFDependencies, "odf-dependencies", "ODF Dependencies", nil)
}

func RookCeph() Profile {
	return odfProfile(OperatorRookCeph, "rook-ceph-operator", "Rook Ceph Operator", []string{"CephCluster"})
}

func CephCSI() Profile {
	return odfProfile(OperatorCephCSI, "cephcsi-operator", "Ceph CSI Operator", []string{"CephConnection"})
}

func OCS() Profile {
	return odfProfile(OperatorOCS, "ocs-operator", "OpenShift Container Storage", []string{"StorageCluster"})
}

func OCSClient() Profile {
	return odfProfile(OperatorOCSClient, "ocs-client-operator", "OCS Client Operator", []string{"StorageClient"})
}

func MCG() Profile {
	return odfProfile(OperatorMCG, "mcg-operator", "Multicloud Object Gateway", []string{"NooBaa"})
}

func ODFCSIAddons() Profile {
	return odfProfile(OperatorODFCSIAddons, "odf-csi-addons-operator", "ODF CSI Addons", []string{"CSIAddonsNode"})
}

func ODFExternalSnapshot() Profile {
	return odfProfile(OperatorODFExternalSnapshot, "odf-external-snapshotter-operator", "ODF External Snapshotter", []string{"VolumeGroupSnapshot"})
}

func ODFPrometheus() Profile {
	return odfProfile(OperatorODFPrometheus, "odf-prometheus-operator", "ODF Prometheus Operator", []string{"Prometheus"})
}

func Recipe() Profile {
	return odfProfile(OperatorRecipe, "recipe", "ODF Recipe Operator", []string{"Recipe"})
}

func odfProfile(id OperatorID, pkg, display string, crKinds []string) Profile {
	if crKinds == nil {
		crKinds = []string{"StorageCluster"}
	}
	return Profile{
		ID:              id,
		Category:        CategoryODF,
		PackageName:     pkg,
		DisplayName:     display,
		DefaultNS:       "openshift-storage",
		LogPatterns:     []string{"StorageCluster", "CephCluster", "NooBaa", "rook", "ceph", "Reconciler error"},
		CRKinds:         crKinds,
	}
}

// --- Backup ---

func OADP() Profile {
	return Profile{
		ID:              OperatorOADP,
		Category:        CategoryBackup,
		PackageName:     "redhat-oadp-operator",
		DisplayName:     "OpenShift API for Data Protection",
		DefaultNS:       "openshift-adp",
		DeploymentNames: []string{"velero", "openshift-adp-controller-manager"},
		LogPatterns:     []string{"backup failed", "restore failed", "BackupStorageLocation", "VolumeSnapshotLocation"},
		CRKinds:         []string{"DataProtectionApplication", "Backup", "Restore", "BackupStorageLocation"},
	}
}
