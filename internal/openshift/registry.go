package openshift

// RepoInfo maps an operator or ClusterOperator to its source repository.
type RepoInfo struct {
	Repo      string
	Component string
}

// RepoRegistry maps ClusterOperator names and OLM package names to GitHub repos.
var RepoRegistry = map[string]RepoInfo{
	// ClusterOperator → repo
	"authentication":                {Repo: "openshift/cluster-authentication-operator", Component: "Authentication"},
	"cloud-controller-manager":      {Repo: "openshift/cluster-cloud-controller-manager-operator", Component: "Cloud CCM"},
	"cloud-credential":              {Repo: "openshift/cloud-credential-operator", Component: "Cloud Credentials"},
	"cluster-autoscaler":            {Repo: "openshift/cluster-autoscaler-operator", Component: "Autoscaler"},
	"config-operator":               {Repo: "openshift/cluster-config-operator", Component: "Config"},
	"console":                       {Repo: "openshift/console-operator", Component: "Console"},
	"csi-snapshot-controller":       {Repo: "openshift/cluster-csi-snapshot-controller-operator", Component: "CSI Snapshot"},
	"dns":                           {Repo: "openshift/cluster-dns-operator", Component: "DNS"},
	"etcd":                          {Repo: "openshift/cluster-etcd-operator", Component: "etcd"},
	"image-registry":                {Repo: "openshift/cluster-image-registry-operator", Component: "Image Registry"},
	"ingress":                       {Repo: "openshift/cluster-ingress-operator", Component: "Ingress"},
	"insights":                      {Repo: "openshift/insights-operator", Component: "Insights"},
	"kube-apiserver":                {Repo: "openshift/cluster-kube-apiserver-operator", Component: "API Server"},
	"kube-controller-manager":       {Repo: "openshift/cluster-kube-controller-manager-operator", Component: "Controller Manager"},
	"kube-scheduler":                {Repo: "openshift/cluster-kube-scheduler-operator", Component: "Scheduler"},
	"kube-storage-version-migrator": {Repo: "openshift/cluster-kube-storage-version-migrator-operator", Component: "Storage Migrator"},
	"machine-api":                   {Repo: "openshift/machine-api-operator", Component: "Machine API"},
	"machine-approver":              {Repo: "openshift/cluster-machine-approver", Component: "Machine Approver"},
	"machine-config":                {Repo: "openshift/machine-config-operator", Component: "Machine Config"},
	"marketplace":                   {Repo: "openshift/operator-marketplace", Component: "Marketplace"},
	"monitoring":                    {Repo: "openshift/cluster-monitoring-operator", Component: "Monitoring"},
	"network":                       {Repo: "openshift/cluster-network-operator", Component: "Network"},
	"node-tuning":                   {Repo: "openshift/cluster-node-tuning-operator", Component: "Node Tuning"},
	"openshift-apiserver":           {Repo: "openshift/cluster-openshift-apiserver-operator", Component: "OpenShift API Server"},
	"openshift-controller-manager":  {Repo: "openshift/cluster-openshift-controller-manager-operator", Component: "Controller Manager"},
	"openshift-samples":             {Repo: "openshift/cluster-samples-operator", Component: "Samples"},
	"operator-lifecycle-manager":    {Repo: "openshift/operator-framework-olm", Component: "OLM"},
	"service-ca":                    {Repo: "openshift/service-ca-operator", Component: "Service CA"},
	"storage":                       {Repo: "openshift/cluster-storage-operator", Component: "Storage"},

	// Platform repos
	"cluster-version": {Repo: "openshift/cluster-version-operator", Component: "CVO"},
	"api":             {Repo: "openshift/api", Component: "API Types"},
	"library-go":      {Repo: "openshift/library-go", Component: "Shared Libraries"},
	"baremetal":        {Repo: "openshift/cluster-baremetal-operator", Component: "Bare Metal"},

	// OLM operators (package name → repo)
	"sriov-network-operator":           {Repo: "openshift/sriov-network-operator", Component: "SR-IOV"},
	"local-storage-operator":           {Repo: "openshift/local-storage-operator", Component: "Local Storage"},
	"ptp-operator":                     {Repo: "openshift/ptp-operator", Component: "PTP"},
	"kubernetes-nmstate-operator":      {Repo: "openshift/kubernetes-nmstate", Component: "NMState"},
	"metallb-operator":                 {Repo: "openshift/metallb-operator", Component: "MetalLB"},
	"numaresources-operator":           {Repo: "openshift-kni/numaresources-operator", Component: "NUMA Resources"},
	"advanced-cluster-management":      {Repo: "stolostron/multiclusterhub-operator", Component: "ACM"},
	"multicluster-engine":              {Repo: "stolostron/backplane-operator", Component: "MCE"},
	"topology-aware-lifecycle-manager": {Repo: "openshift-kni/cluster-group-upgrades-operator", Component: "TALM"},
	"redhat-oadp-operator":             {Repo: "openshift/oadp-operator", Component: "OADP"},
	"lvms-operator":                    {Repo: "openshift/lvm-operator", Component: "LVMS"},
	"odf-operator":                     {Repo: "red-hat-storage/odf-operator", Component: "ODF"},
	"cluster-logging":                  {Repo: "openshift/cluster-logging-operator", Component: "Logging"},
	"openshift-gitops-operator":        {Repo: "redhat-developer/gitops-operator", Component: "GitOps"},
	"openshift-cert-manager-operator":  {Repo: "openshift/cert-manager-operator", Component: "Cert Manager"},
	"lifecycle-agent":                  {Repo: "openshift-kni/lifecycle-agent", Component: "Lifecycle Agent"},
}

// LookupRepo finds the source repository for an operator or ClusterOperator name.
func LookupRepo(name string) (RepoInfo, bool) {
	info, ok := RepoRegistry[name]
	return info, ok
}

// LookupByComponent finds repos matching a component name (case-insensitive prefix).
func LookupByComponent(component string) []RepoInfo {
	var results []RepoInfo
	for _, info := range RepoRegistry {
		if info.Component == component {
			results = append(results, info)
		}
	}
	return results
}

// RepoURL returns the full GitHub URL for a repo path.
func RepoURL(repoPath string) string {
	return "https://github.com/" + repoPath
}

// AllRepos returns every registered repo path.
func AllRepos() []string {
	seen := make(map[string]bool)
	var repos []string
	for _, info := range RepoRegistry {
		if !seen[info.Repo] {
			seen[info.Repo] = true
			repos = append(repos, info.Repo)
		}
	}
	return repos
}
