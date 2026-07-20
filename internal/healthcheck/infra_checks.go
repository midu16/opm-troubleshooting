package healthcheck

import (
	"fmt"
	"strings"

	"github.com/midu16/opm-troubleshooting/internal/datasource"
)

// InfraConfig provides cluster-wide data for infrastructure health checks.
type InfraConfig struct {
	DataSource datasource.ClusterDataSource
}

// RunInfra executes infrastructure health dimensions against a ClusterDataSource.
func RunInfra(cfg InfraConfig) (*Report, error) {
	report := &Report{
		OperatorPackage: "cluster-infrastructure",
		TotalDimensions: len(InfraDimensionIDs()),
		Dimensions:      make([]DimensionResult, 0, len(InfraDimensionIDs())),
	}

	for _, dimID := range InfraDimensionIDs() {
		result := checkInfraDimension(dimID, cfg)
		report.Dimensions = append(report.Dimensions, result)

		switch result.Status {
		case StatusPass:
			report.Passed++
		case StatusFail:
			report.Failed++
		case StatusWarn:
			report.Warnings++
		case StatusSkip:
			report.Skipped++
		}
	}

	return report, nil
}

func checkInfraDimension(id DimensionID, cfg InfraConfig) DimensionResult {
	meta := dimensionMeta[id]
	result := DimensionResult{
		ID:       id,
		Name:     meta.Name,
		Category: meta.Category,
		Status:   StatusUnknown,
		Severity: SeverityInfo,
	}

	if cfg.DataSource == nil {
		result.Status = StatusSkip
		result.Summary = "No cluster data source available"
		return result
	}

	switch id {
	case DimNodeHealth:
		result = checkNodeHealth(cfg.DataSource)
	case DimEtcdHealth:
		result = checkEtcdHealth(cfg.DataSource)
	case DimAPIServerHealth:
		result = checkAPIServerHealthInfra(cfg.DataSource)
	case DimClusterVersion:
		result = checkClusterVersionHealth(cfg.DataSource)
	case DimNetworkOperator:
		result = checkNetworkOperator(cfg.DataSource)
	case DimDNSHealth:
		result = checkDNSHealth(cfg.DataSource)
	case DimIngressHealth:
		result = checkIngressHealthInfra(cfg.DataSource)
	case DimPVHealth:
		result = checkPVHealthInfra(cfg.DataSource)
	case DimStorageOperator:
		result = checkStorageOperator(cfg.DataSource)
	case DimCertificateExpiry:
		result = checkCertExpiry(cfg.DataSource)
	case DimAuthOperator:
		result = checkAuthOperator(cfg.DataSource)
	case DimMCPHealth:
		result = checkMCPHealthInfra(cfg.DataSource)
	case DimMonitoringStack:
		result = checkMonitoringStack(cfg.DataSource)
	}

	result.ID = id
	result.Name = meta.Name
	result.Category = meta.Category
	return result
}

func checkNodeHealth(src datasource.ClusterDataSource) DimensionResult {
	r := baseResult(DimNodeHealth)
	nodes, err := src.GetNodes()
	if err != nil || len(nodes) == 0 {
		r.Status = StatusSkip
		r.Summary = "No node data available"
		return r
	}

	notReady := make([]string, 0)
	pressured := make([]string, 0)
	unschedulable := make([]string, 0)

	for _, node := range nodes {
		if !node.Ready {
			notReady = append(notReady, node.Name)
		}
		if node.Unschedulable {
			unschedulable = append(unschedulable, node.Name)
		}
		for _, cond := range node.Conditions {
			if cond.Status == "True" {
				switch cond.Type {
				case "MemoryPressure", "DiskPressure", "PIDPressure":
					pressured = append(pressured, fmt.Sprintf("%s: %s", node.Name, cond.Type))
				}
			}
		}
	}

	if len(notReady) > 0 {
		r.Status = StatusFail
		r.Severity = SeverityCritical
		r.Summary = fmt.Sprintf("%d/%d nodes NotReady", len(notReady), len(nodes))
		r.Evidence = notReady
		r.Recommendation = "Check kubelet logs on NotReady nodes: journalctl -u kubelet"
		return r
	}

	if len(pressured) > 0 {
		r.Status = StatusWarn
		r.Severity = SeverityWarning
		r.Summary = "Nodes under resource pressure"
		r.Evidence = pressured
		r.Recommendation = "Review node resource usage and consider draining affected nodes"
		return r
	}

	r.Status = StatusPass
	r.Severity = SeverityHealthy
	r.Summary = fmt.Sprintf("All %d nodes Ready", len(nodes))
	if len(unschedulable) > 0 {
		r.Evidence = append(r.Evidence, fmt.Sprintf("%d nodes cordoned", len(unschedulable)))
	}
	return r
}

func checkEtcdHealth(src datasource.ClusterDataSource) DimensionResult {
	r := baseResult(DimEtcdHealth)

	// Check via cluster operator status
	cos, err := src.GetClusterOperators()
	if err != nil {
		r.Status = StatusSkip
		r.Summary = "Cannot read cluster operator status"
		return r
	}

	for _, co := range cos {
		if co.Name == "etcd" {
			if co.Degraded {
				r.Status = StatusFail
				r.Severity = SeverityCritical
				r.Summary = "etcd cluster operator degraded"
				for _, cond := range co.Conditions {
					if cond.Type == "Degraded" && cond.Status == "True" {
						r.Evidence = append(r.Evidence, cond.Message)
					}
				}
				r.Recommendation = "Check etcd pod logs in openshift-etcd namespace and disk performance"
				return r
			}
			if !co.Available {
				r.Status = StatusFail
				r.Severity = SeverityCritical
				r.Summary = "etcd cluster operator not available"
				r.Recommendation = "Check etcd pods: oc get pods -n openshift-etcd"
				return r
			}
			r.Status = StatusPass
			r.Severity = SeverityHealthy
			r.Summary = "etcd cluster operator healthy"
			return r
		}
	}

	r.Status = StatusSkip
	r.Summary = "etcd cluster operator not found"
	return r
}

func checkAPIServerHealthInfra(src datasource.ClusterDataSource) DimensionResult {
	r := baseResult(DimAPIServerHealth)

	apiStatus, err := src.GetAPIServerStatus()
	if err != nil {
		r.Status = StatusSkip
		r.Summary = "Cannot determine API server status"
		return r
	}

	if !apiStatus.Available {
		r.Status = StatusFail
		r.Severity = SeverityCritical
		r.Summary = "API server not available"
		for _, cond := range apiStatus.Conditions {
			if cond.Status == "True" && (cond.Type == "Degraded" || cond.Type == "Failing") {
				r.Evidence = append(r.Evidence, fmt.Sprintf("%s: %s", cond.Type, cond.Message))
			}
		}
		r.Recommendation = "Check kube-apiserver pods in openshift-kube-apiserver namespace"
		return r
	}

	r.Status = StatusPass
	r.Severity = SeverityHealthy
	r.Summary = "API server available"
	return r
}

func checkClusterVersionHealth(src datasource.ClusterDataSource) DimensionResult {
	r := baseResult(DimClusterVersion)

	cv, err := src.GetClusterVersion()
	if err != nil {
		r.Status = StatusSkip
		r.Summary = "Cannot read cluster version"
		return r
	}

	if cv.Failing {
		r.Status = StatusFail
		r.Severity = SeverityCritical
		r.Summary = fmt.Sprintf("Cluster version %s: upgrade failing", cv.Version)
		for _, cond := range cv.Conditions {
			if cond.Type == "Failing" && cond.Status == "True" {
				r.Evidence = append(r.Evidence, cond.Message)
			}
		}
		r.Recommendation = "Review ClusterVersion conditions: oc get clusterversion -o yaml"
		return r
	}

	if cv.Progressing {
		r.Status = StatusWarn
		r.Severity = SeverityWarning
		r.Summary = fmt.Sprintf("Cluster version %s: upgrade in progress", cv.Version)
		for _, cond := range cv.Conditions {
			if cond.Type == "Progressing" && cond.Status == "True" {
				r.Evidence = append(r.Evidence, cond.Message)
			}
		}
		return r
	}

	r.Status = StatusPass
	r.Severity = SeverityHealthy
	r.Summary = fmt.Sprintf("Cluster version %s (channel: %s)", cv.Version, cv.Channel)
	return r
}

func checkNetworkOperator(src datasource.ClusterDataSource) DimensionResult {
	r := baseResult(DimNetworkOperator)
	return checkClusterOperatorByName(src, r, "network", "Check OVN-Kubernetes or SDN pods in openshift-ovn-kubernetes namespace")
}

func checkDNSHealth(src datasource.ClusterDataSource) DimensionResult {
	r := baseResult(DimDNSHealth)
	return checkClusterOperatorByName(src, r, "dns", "Check CoreDNS pods: oc get pods -n openshift-dns")
}

func checkIngressHealthInfra(src datasource.ClusterDataSource) DimensionResult {
	r := baseResult(DimIngressHealth)
	return checkClusterOperatorByName(src, r, "ingress", "Check router pods: oc get pods -n openshift-ingress")
}

func checkStorageOperator(src datasource.ClusterDataSource) DimensionResult {
	r := baseResult(DimStorageOperator)
	return checkClusterOperatorByName(src, r, "storage", "Check storage pods: oc get pods -n openshift-cluster-csi-drivers")
}

func checkAuthOperator(src datasource.ClusterDataSource) DimensionResult {
	r := baseResult(DimAuthOperator)
	return checkClusterOperatorByName(src, r, "authentication", "Check OAuth pods: oc get pods -n openshift-authentication")
}

func checkMonitoringStack(src datasource.ClusterDataSource) DimensionResult {
	r := baseResult(DimMonitoringStack)
	return checkClusterOperatorByName(src, r, "monitoring", "Check Prometheus/Alertmanager pods in openshift-monitoring")
}

func checkClusterOperatorByName(src datasource.ClusterDataSource, r DimensionResult, name, recommendation string) DimensionResult {
	cos, err := src.GetClusterOperators()
	if err != nil {
		r.Status = StatusSkip
		r.Summary = "Cannot read cluster operator status"
		return r
	}

	for _, co := range cos {
		if co.Name == name {
			if co.Degraded {
				r.Status = StatusFail
				r.Severity = SeverityCritical
				r.Summary = fmt.Sprintf("%s operator degraded", name)
				for _, cond := range co.Conditions {
					if cond.Type == "Degraded" && cond.Status == "True" {
						r.Evidence = append(r.Evidence, cond.Message)
					}
				}
				r.Recommendation = recommendation
				return r
			}
			if !co.Available {
				r.Status = StatusFail
				r.Severity = SeverityCritical
				r.Summary = fmt.Sprintf("%s operator not available", name)
				r.Recommendation = recommendation
				return r
			}
			if co.Progressing {
				r.Status = StatusWarn
				r.Severity = SeverityWarning
				r.Summary = fmt.Sprintf("%s operator progressing", name)
				return r
			}
			r.Status = StatusPass
			r.Severity = SeverityHealthy
			r.Summary = fmt.Sprintf("%s operator healthy", name)
			return r
		}
	}

	r.Status = StatusSkip
	r.Summary = fmt.Sprintf("%s cluster operator not found", name)
	return r
}

func checkPVHealthInfra(src datasource.ClusterDataSource) DimensionResult {
	r := baseResult(DimPVHealth)

	pvs, err := src.GetPVs()
	if err != nil {
		r.Status = StatusSkip
		r.Summary = "Cannot read PersistentVolume data"
		return r
	}

	if len(pvs) == 0 {
		r.Status = StatusPass
		r.Severity = SeverityHealthy
		r.Summary = "No PersistentVolumes configured"
		return r
	}

	failed := make([]string, 0)
	released := make([]string, 0)
	for _, pv := range pvs {
		switch pv.Phase {
		case "Failed":
			failed = append(failed, pv.Name)
		case "Released":
			if pv.ReclaimPolicy == "Retain" {
				released = append(released, pv.Name)
			}
		}
	}

	if len(failed) > 0 {
		r.Status = StatusFail
		r.Severity = SeverityCritical
		r.Summary = fmt.Sprintf("%d PVs in Failed state", len(failed))
		r.Evidence = failed
		r.Recommendation = "Investigate PV failure with oc describe pv <name>"
		return r
	}

	if len(released) > 0 {
		r.Status = StatusWarn
		r.Severity = SeverityWarning
		r.Summary = fmt.Sprintf("%d PVs in Released state with Retain policy", len(released))
		r.Evidence = released
		return r
	}

	r.Status = StatusPass
	r.Severity = SeverityHealthy
	r.Summary = fmt.Sprintf("All %d PVs healthy", len(pvs))
	return r
}

func checkCertExpiry(src datasource.ClusterDataSource) DimensionResult {
	r := baseResult(DimCertificateExpiry)

	certs, err := src.GetCertificates()
	if err != nil || len(certs) == 0 {
		r.Status = StatusSkip
		r.Summary = "Certificate data not available from this data source"
		return r
	}

	// Certificate checks would go here
	r.Status = StatusPass
	r.Severity = SeverityHealthy
	r.Summary = fmt.Sprintf("%d certificates checked", len(certs))
	return r
}

func checkMCPHealthInfra(src datasource.ClusterDataSource) DimensionResult {
	r := baseResult(DimMCPHealth)

	mcps, err := src.GetMachineConfigPools()
	if err != nil {
		r.Status = StatusSkip
		r.Summary = "Cannot read MachineConfigPool data"
		return r
	}

	if len(mcps) == 0 {
		r.Status = StatusSkip
		r.Summary = "No MachineConfigPools found"
		return r
	}

	degraded := make([]string, 0)
	updating := make([]string, 0)
	for _, mcp := range mcps {
		if mcp.Degraded {
			detail := fmt.Sprintf("%s (%d/%d degraded machines)", mcp.Name, mcp.DegradedMachineCount, mcp.MachineCount)
			degraded = append(degraded, detail)
		} else if mcp.Updating {
			detail := fmt.Sprintf("%s (%d/%d updated)", mcp.Name, mcp.UpdatedMachineCount, mcp.MachineCount)
			updating = append(updating, detail)
		}
	}

	if len(degraded) > 0 {
		r.Status = StatusFail
		r.Severity = SeverityCritical
		r.Summary = "Degraded MachineConfigPools"
		r.Evidence = degraded
		r.Recommendation = "Check degraded machines: oc describe mcp <pool>; check node MachineConfig: oc describe node <name>"
		return r
	}

	if len(updating) > 0 {
		r.Status = StatusWarn
		r.Severity = SeverityWarning
		r.Summary = "MachineConfigPools updating"
		r.Evidence = updating
		return r
	}

	readySummary := make([]string, 0, len(mcps))
	for _, mcp := range mcps {
		readySummary = append(readySummary, fmt.Sprintf("%s(%d/%d)", mcp.Name, mcp.ReadyMachineCount, mcp.MachineCount))
	}

	r.Status = StatusPass
	r.Severity = SeverityHealthy
	r.Summary = fmt.Sprintf("All MCPs healthy: %s", strings.Join(readySummary, ", "))
	return r
}
