package ingest

import (
	"github.com/midu16/opm-troubleshooting/internal/rag"
)

// knownIssue holds the data for a single known operator issue entry.
type knownIssue struct {
	IssueID     string
	Operator    string
	Summary     string
	Workaround  string
	FixVersion  string
	OCPVersions string
}

// knownIssuesDB is a hardcoded database of known OpenShift operator issues.
var knownIssuesDB = []knownIssue{
	{
		IssueID:     "KNOWN-ETCD-001",
		Operator:    "cluster-etcd-operator",
		Summary:     "Etcd cluster degrades during single-node control plane replacement. When a control plane node is replaced, the etcd operator may fail to properly remove the old member and add the new one, leading to a degraded etcd cluster that prevents cluster updates.",
		Workaround:  "Manually remove the stale etcd member using 'etcdctl member remove' from a healthy etcd pod, then force the etcd operator to reconcile by deleting the etcd operator pod.",
		FixVersion:  "4.22.3",
		OCPVersions: "4.22.0-4.22.2",
	},
	{
		IssueID:     "OCPBUGS-12345",
		Operator:    "machine-config-operator",
		Summary:     "MachineConfigPool stuck in Updating state after applying custom MachineConfig with kernel arguments. The MCO fails to drain nodes when PodDisruptionBudgets block eviction, causing an indefinite loop.",
		Workaround:  "Temporarily remove the PDB blocking eviction or add the annotation 'unsupported-drain: true' to the MachineConfigPool to bypass drain.",
		FixVersion:  "4.22.4",
		OCPVersions: "4.22.0-4.22.3",
	},
	{
		IssueID:     "OCPBUGS-23456",
		Operator:    "cluster-network-operator",
		Summary:     "OVN-Kubernetes pods crash-loop on dual-stack clusters when node IPv6 address assignment is delayed during boot. The CNO marks the network operator as Degraded.",
		Workaround:  "Ensure IPv6 addresses are assigned before kubelet starts by configuring NetworkManager to wait for IPv6 DAD completion. Alternatively, restart the affected OVN-Kubernetes pods.",
		FixVersion:  "4.22.2",
		OCPVersions: "4.22.0-4.22.1",
	},
	{
		IssueID:     "OCPBUGS-34567",
		Operator:    "cluster-storage-operator",
		Summary:     "CSI driver pods fail to start on RHEL 9.4 nodes due to missing SELinux policy for CSI socket directory. The storage operator reports Degraded condition with 'CSIDriverStartFailed' reason.",
		Workaround:  "Apply a custom SELinux policy module that allows the CSI driver to create and bind to the socket path, or set the node SELinux mode to permissive temporarily.",
		FixVersion:  "4.22.3",
		OCPVersions: "4.22.0-4.22.2",
	},
	{
		IssueID:     "KNOWN-SRIOV-001",
		Operator:    "sriov-network-operator",
		Summary:     "SR-IOV VFs not created after node reboot on Intel E810 NICs with certain firmware versions. The sriov-network-config-daemon fails to configure VFs and logs 'device or resource busy' errors.",
		Workaround:  "Update Intel E810 NIC firmware to version 4.20 or later. As a temporary fix, unbind and rebind the PF driver after boot by running 'echo 0 > /sys/class/net/<pf>/device/sriov_numvfs && echo <num> > /sys/class/net/<pf>/device/sriov_numvfs'.",
		FixVersion:  "4.22.5",
		OCPVersions: "4.22.0-4.22.4",
	},
	{
		IssueID:     "KNOWN-PTP-001",
		Operator:    "ptp-operator",
		Summary:     "PTP grandmaster clock synchronization lost after leap second event. The linuxptp daemon fails to re-synchronize and the PTP operator reports Degraded with clock offset exceeding threshold.",
		Workaround:  "Restart the linuxptp-daemon pods on affected nodes. Configure the PTP profile with 'step_threshold=1' to allow clock stepping after large offsets.",
		FixVersion:  "4.22.4",
		OCPVersions: "4.22.0-4.22.3",
	},
	{
		IssueID:     "OCPBUGS-45678",
		Operator:    "cluster-monitoring-operator",
		Summary:     "Prometheus persistent storage fills up and enters CrashLoopBackOff when custom recording rules generate high-cardinality metrics. The monitoring stack becomes unavailable.",
		Workaround:  "Increase PVC size for Prometheus using the cluster-monitoring-config ConfigMap. Remove or optimize high-cardinality recording rules. Use 'oc exec' to run TSDB compaction manually.",
		FixVersion:  "4.22.3",
		OCPVersions: "4.22.0-4.22.2",
	},
	{
		IssueID:     "OCPBUGS-56789",
		Operator:    "cluster-authentication-operator",
		Summary:     "OAuth server pods fail to roll out when the cluster ingress certificate is renewed. The authentication operator gets stuck in Progressing state due to a stale CA bundle reference.",
		Workaround:  "Delete the oauth-openshift deployment and let the authentication operator recreate it. Alternatively, manually update the CA bundle ConfigMap in the openshift-authentication namespace.",
		FixVersion:  "4.22.2",
		OCPVersions: "4.22.0-4.22.1",
	},
	{
		IssueID:     "OCPBUGS-67890",
		Operator:    "cluster-ingress-operator",
		Summary:     "IngressController custom routes return 503 errors after cluster upgrade when haproxy reload fails silently. The router pods report Ready but do not serve traffic for newly added routes.",
		Workaround:  "Delete the affected router pods to force a clean restart. If using custom haproxy templates, validate them against the new haproxy version shipped with the upgrade.",
		FixVersion:  "4.22.3",
		OCPVersions: "4.22.0-4.22.2",
	},
	{
		IssueID:     "KNOWN-LSO-001",
		Operator:    "local-storage-operator",
		Summary:     "LocalVolumeDiscovery does not detect NVMe drives on certain server models where NVMe device paths use non-standard naming. The LSO tolerations prevent discovery pods from running on infra nodes.",
		Workaround:  "Create a LocalVolume CR with explicit device paths instead of relying on auto-discovery. Add the correct tolerations to the LocalVolumeDiscovery CR.",
		FixVersion:  "4.22.4",
		OCPVersions: "4.22.0-4.22.3",
	},
	{
		IssueID:     "KNOWN-NTO-001",
		Operator:    "cluster-node-tuning-operator",
		Summary:     "PerformanceProfile does not apply hugepages configuration on real-time kernel nodes when NUMA topology changes after a BIOS update. The NTO logs 'topology mismatch' warnings.",
		Workaround:  "Delete and recreate the PerformanceProfile after the BIOS update. Verify NUMA topology matches the profile using 'lscpu' and 'numactl --hardware' on the affected node.",
		FixVersion:  "4.22.3",
		OCPVersions: "4.22.0-4.22.2",
	},
	{
		IssueID:     "KNOWN-CVO-001",
		Operator:    "cluster-version-operator",
		Summary:     "Cluster upgrade stalls at 60% when a ClusterOperator reports Available=False due to a transient API server connectivity issue. The CVO does not retry the precondition check and marks the upgrade as failed.",
		Workaround:  "If the underlying ClusterOperator issue is resolved, delete the CVO pod to restart the upgrade. Alternatively, use 'oc adm upgrade' with '--force' to bypass the stalled precondition.",
		FixVersion:  "4.22.2",
		OCPVersions: "4.22.0-4.22.1",
	},
}

// BuildKnownIssues returns a slice of Documents representing the hardcoded
// database of known OpenShift operator issues.
func BuildKnownIssues() []rag.Document {
	docs := make([]rag.Document, 0, len(knownIssuesDB))

	for _, issue := range knownIssuesDB {
		content := issue.Summary
		if issue.Workaround != "" {
			content += "\n\nWorkaround: " + issue.Workaround
		}

		docs = append(docs, rag.Document{
			ID:      "issue-" + issue.IssueID,
			Content: content,
			Metadata: map[string]string{
				"issue_id":    issue.IssueID,
				"operator":    issue.Operator,
				"workaround":  issue.Workaround,
				"fix_version": issue.FixVersion,
				"type":        "known_issue",
				"ocp_version": issue.OCPVersions,
			},
		})
	}
	return docs
}
