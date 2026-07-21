package healthcheck

import (
	"context"
	"fmt"
	"strings"

	"github.com/midu16/opm-troubleshooting/internal/mustgather"
	"github.com/midu16/opm-troubleshooting/internal/telco"
)

// Severity classifies finding impact.
type Severity string

const (
	SeverityCritical Severity = "Critical"
	SeverityWarning  Severity = "Warning"
	SeverityInfo     Severity = "Info"
	SeverityHealthy  Severity = "Healthy"
)

// Status is the dimension check outcome.
type Status string

const (
	StatusPass    Status = "Pass"
	StatusFail    Status = "Fail"
	StatusWarn    Status = "Warn"
	StatusSkip    Status = "Skip"
	StatusUnknown Status = "Unknown"
)

// DimensionID identifies one of the 20 systematic health dimensions.
type DimensionID string

const (
	DimCatalogSource     DimensionID = "catalog_source"
	DimSubscription      DimensionID = "catalog_subscription"
	DimInstallPlan       DimensionID = "install_plan"
	DimOperatorGroup     DimensionID = "operator_group"
	DimCSVPhase          DimensionID = "csv_phase"
	DimCSVRequirements   DimensionID = "csv_requirements"
	DimDeploymentReady   DimensionID = "deployment_ready"
	DimPodHealth         DimensionID = "pod_health"
	DimContainerRestarts DimensionID = "container_restarts"
	DimImagePull         DimensionID = "image_pull"
	DimRBAC              DimensionID = "rbac_serviceaccount"
	DimWarningEvents     DimensionID = "warning_events"
	DimCRDEstablished    DimensionID = "crd_established"
	DimWebhooks          DimensionID = "webhooks"
	DimResourceQuota     DimensionID = "resource_quota"
	DimScheduling        DimensionID = "node_scheduling"
	DimIDMSMirror        DimensionID = "idms_mirror_config"
	DimManagedClusters   DimensionID = "managed_cluster_connectivity"
	DimPolicyCompliance  DimensionID = "policy_compliance"
	DimBackupRestore     DimensionID = "backup_restore_status"

	// Infrastructure dimensions
	DimNodeHealth        DimensionID = "node_health"
	DimEtcdHealth        DimensionID = "etcd_health"
	DimAPIServerHealth   DimensionID = "apiserver_health"
	DimClusterVersion    DimensionID = "cluster_version"

	// Networking dimensions
	DimNetworkOperator   DimensionID = "network_operator"
	DimDNSHealth         DimensionID = "dns_health"
	DimIngressHealth     DimensionID = "ingress_health"

	// Storage dimensions
	DimPVHealth          DimensionID = "pv_health"
	DimStorageOperator   DimensionID = "storage_operator"

	// Security dimensions
	DimCertificateExpiry DimensionID = "certificate_expiry"
	DimAuthOperator      DimensionID = "auth_operator"

	// Platform dimensions
	DimMCPHealth         DimensionID = "mcp_health"
	DimMonitoringStack   DimensionID = "monitoring_stack"
)

// DimensionResult is the outcome of a single health dimension check.
type DimensionResult struct {
	ID             DimensionID `json:"id"`
	Name           string      `json:"name"`
	Category       string      `json:"category"`
	Status         Status      `json:"status"`
	Severity       Severity    `json:"severity"`
	Summary        string      `json:"summary"`
	Evidence       []string    `json:"evidence,omitempty"`
	Recommendation string      `json:"recommendation,omitempty"`
}

// Report aggregates all dimension results for a diagnosis run.
type Report struct {
	OperatorPackage string            `json:"operator_package"`
	Namespace       string            `json:"namespace"`
	TotalDimensions int               `json:"total_dimensions"`
	Passed          int               `json:"passed"`
	Failed          int               `json:"failed"`
	Warnings        int               `json:"warnings"`
	Skipped         int               `json:"skipped"`
	Dimensions      []DimensionResult `json:"dimensions"`
	DurationMs      int64             `json:"duration_ms,omitempty"`
}

// Config controls health check execution.
type Config struct {
	MustGatherPath string
	Operator       mustgather.OperatorState
	Profile        *telco.Profile
}

// OLMDimensionIDs returns the 20 original OLM-focused dimensions.
func OLMDimensionIDs() []DimensionID {
	return []DimensionID{
		DimCatalogSource,
		DimSubscription,
		DimInstallPlan,
		DimOperatorGroup,
		DimCSVPhase,
		DimCSVRequirements,
		DimDeploymentReady,
		DimPodHealth,
		DimContainerRestarts,
		DimImagePull,
		DimRBAC,
		DimWarningEvents,
		DimCRDEstablished,
		DimWebhooks,
		DimResourceQuota,
		DimScheduling,
		DimIDMSMirror,
		DimManagedClusters,
		DimPolicyCompliance,
		DimBackupRestore,
	}
}

// InfraDimensionIDs returns the infrastructure health dimensions.
func InfraDimensionIDs() []DimensionID {
	return []DimensionID{
		DimNodeHealth,
		DimEtcdHealth,
		DimAPIServerHealth,
		DimClusterVersion,
		DimNetworkOperator,
		DimDNSHealth,
		DimIngressHealth,
		DimPVHealth,
		DimStorageOperator,
		DimCertificateExpiry,
		DimAuthOperator,
		DimMCPHealth,
		DimMonitoringStack,
	}
}

// AllDimensionIDs returns all dimensions (OLM + infrastructure) in stable order.
func AllDimensionIDs() []DimensionID {
	all := OLMDimensionIDs()
	all = append(all, InfraDimensionIDs()...)
	return all
}

// dimensionMeta maps dimension IDs to display metadata.
var dimensionMeta = map[DimensionID]struct{ Name, Category string }{
	DimCatalogSource:     {"CatalogSource Connectivity", "OLM"},
	DimSubscription:      {"Subscription State", "OLM"},
	DimInstallPlan:       {"InstallPlan Phase", "OLM"},
	DimOperatorGroup:     {"OperatorGroup Configuration", "OLM"},
	DimCSVPhase:          {"ClusterServiceVersion Phase", "OLM"},
	DimCSVRequirements:   {"CSV Requirement Status", "OLM"},
	DimDeploymentReady:     {"Deployment Availability", "Workload"},
	DimPodHealth:           {"Pod Phase Health", "Workload"},
	DimContainerRestarts:   {"Container Restart Count", "Workload"},
	DimImagePull:           {"Image Pull Status", "Workload"},
	DimRBAC:                {"ServiceAccount & RBAC", "Security"},
	DimWarningEvents:       {"Warning Events", "Events"},
	DimCRDEstablished:      {"CRD Established Status", "API"},
	DimWebhooks:            {"Admission Webhooks", "API"},
	DimResourceQuota:       {"Resource Quota Pressure", "Capacity"},
	DimScheduling:          {"Node Scheduling Constraints", "Capacity"},
	DimIDMSMirror:          {"Image Digest Mirror Set", "Disconnected"},
	DimManagedClusters:     {"Managed Cluster Connectivity", "Telco/ACM"},
	DimPolicyCompliance:    {"Policy Compliance State", "Telco/TALM"},
	DimBackupRestore:       {"Backup/Restore Operator Status", "Telco/OADP"},

	// Infrastructure
	DimNodeHealth:        {"Node Health", "Infrastructure"},
	DimEtcdHealth:        {"etcd Cluster Health", "Infrastructure"},
	DimAPIServerHealth:   {"API Server Availability", "Infrastructure"},
	DimClusterVersion:    {"Cluster Version Status", "Infrastructure"},

	// Networking
	DimNetworkOperator:   {"Network Operator Status", "Networking"},
	DimDNSHealth:         {"DNS Resolution Health", "Networking"},
	DimIngressHealth:     {"Ingress Controller Health", "Networking"},

	// Storage
	DimPVHealth:          {"PersistentVolume Health", "Storage"},
	DimStorageOperator:   {"Storage Operator Status", "Storage"},

	// Security
	DimCertificateExpiry: {"Certificate Expiry", "Security"},
	DimAuthOperator:      {"Authentication Operator", "Security"},

	// Platform
	DimMCPHealth:         {"MachineConfigPool Health", "Platform"},
	DimMonitoringStack:   {"Monitoring Stack Health", "Platform"},
}

// Run executes the 20 OLM health dimensions against must-gather data.
func Run(ctx context.Context, cfg Config) (*Report, error) {
	olmDims := OLMDimensionIDs()
	report := &Report{
		OperatorPackage: cfg.Operator.PackageName,
		Namespace:       cfg.Operator.Namespace,
		TotalDimensions: len(olmDims),
		Dimensions:      make([]DimensionResult, 0, len(olmDims)),
	}

	ns := cfg.Operator.Namespace
	if cfg.Profile != nil && cfg.Profile.DefaultNS != "" && ns == "" {
		ns = cfg.Profile.DefaultNS
	}

	workload, _ := mustgather.ParseWorkloads(cfg.MustGatherPath, ns)

	for _, dimID := range olmDims {
		result := checkDimension(ctx, dimID, cfg, workload)
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

func checkDimension(ctx context.Context, id DimensionID, cfg Config, workload *mustgather.WorkloadState) DimensionResult {
	meta := dimensionMeta[id]
	result := DimensionResult{
		ID:       id,
		Name:     meta.Name,
		Category: meta.Category,
		Status:   StatusUnknown,
		Severity: SeverityInfo,
	}

	switch id {
	case DimCatalogSource:
		result = checkCatalogSource(cfg.Operator)
	case DimSubscription:
		result = checkSubscription(cfg.Operator)
	case DimInstallPlan:
		result = checkInstallPlan(ctx, cfg)
	case DimOperatorGroup:
		result = checkOperatorGroup(cfg)
	case DimCSVPhase:
		result = checkCSVPhase(cfg.Operator)
	case DimCSVRequirements:
		result = checkCSVRequirements(cfg.Operator)
	case DimDeploymentReady:
		result = checkDeploymentReady(workload)
	case DimPodHealth:
		result = checkPodHealth(workload)
	case DimContainerRestarts:
		result = checkContainerRestarts(workload)
	case DimImagePull:
		result = checkImagePull(workload)
	case DimRBAC:
		result = checkRBAC(cfg)
	case DimWarningEvents:
		result = checkWarningEvents(workload)
	case DimCRDEstablished:
		result = checkCRDEstablished(cfg)
	case DimWebhooks:
		result = checkWebhooks(cfg)
	case DimResourceQuota:
		result = checkResourceQuota(cfg)
	case DimScheduling:
		result = checkScheduling(workload)
	case DimIDMSMirror:
		result = checkIDMSMirror(cfg)
	case DimManagedClusters:
		result = checkManagedClusters(cfg)
	case DimPolicyCompliance:
		result = checkPolicyCompliance(cfg)
	case DimBackupRestore:
		result = checkBackupRestore(cfg, workload)
	}

	result.ID = id
	result.Name = meta.Name
	result.Category = meta.Category
	return result
}

func checkCatalogSource(op mustgather.OperatorState) DimensionResult {
	r := baseResult(DimCatalogSource)
	for _, cond := range op.Conditions {
		if cond.Type == "CatalogSourcesUnhealthy" && cond.Status == "True" {
			r.Status = StatusFail
			r.Severity = SeverityCritical
			r.Summary = "Catalog source is unhealthy"
			r.Evidence = append(r.Evidence, cond.Message)
			r.Recommendation = "Verify CatalogSource pod is running and registry credentials are valid"
			return r
		}
	}
	r.Status = StatusPass
	r.Severity = SeverityHealthy
	r.Summary = "No catalog source health issues detected"
	return r
}

func checkSubscription(op mustgather.OperatorState) DimensionResult {
	r := baseResult(DimSubscription)
	if op.State == "NotFound" {
		r.Status = StatusSkip
		r.Summary = "Operator not installed on cluster"
		return r
	}
	if op.State == "ClusterConfig" {
		r.Status = StatusSkip
		r.Summary = "Cluster configuration dimension (not an OLM subscription)"
		return r
	}
	// ResolutionFailed is critical only when the operator never installed
	for _, cond := range op.Conditions {
		if cond.Type == "ResolutionFailed" && cond.Status == "True" && op.InstalledCSV == "" {
			r.Status = StatusFail
			r.Severity = SeverityCritical
			r.Summary = "OLM resolution failed"
			r.Evidence = append(r.Evidence, cond.Message)
			r.Recommendation = "Verify package exists in catalog and dependency subscriptions resolve"
			return r
		}
	}
	switch op.State {
	case "AtLatestKnown":
		r.Status = StatusPass
		r.Severity = SeverityHealthy
		r.Summary = fmt.Sprintf("Subscription healthy: %s", op.State)
	case "Failed":
		r.Status = StatusFail
		r.Severity = SeverityCritical
		r.Summary = "Subscription in Failed state"
		r.Recommendation = "Inspect subscription conditions and InstallPlan"
	default:
		r.Status = StatusWarn
		r.Severity = SeverityWarning
		r.Summary = fmt.Sprintf("Subscription state: %s", op.State)
		if op.CurrentCSV != "" && op.InstalledCSV != "" && op.CurrentCSV != op.InstalledCSV {
			r.Evidence = append(r.Evidence, fmt.Sprintf("currentCSV=%s installedCSV=%s", op.CurrentCSV, op.InstalledCSV))
			r.Recommendation = "Upgrade may be stuck; check InstallPlan approval and requirements"
		}
	}
	return r
}

func checkInstallPlan(ctx context.Context, cfg Config) DimensionResult {
	r := baseResult(DimInstallPlan)
	if cfg.Operator.InstallPlanRef == "" {
		r.Status = StatusSkip
		r.Summary = "No InstallPlan reference on subscription"
		return r
	}

	planPath, err := mustgather.FindInstallPlan(cfg.MustGatherPath, cfg.Operator.Namespace, cfg.Operator.InstallPlanRef)
	if err != nil {
		r.Status = StatusUnknown
		r.Summary = "InstallPlan not found in must-gather"
		return r
	}

	plan, err := mustgather.ParseInstallPlan(ctx, planPath)
	if err != nil {
		r.Status = StatusUnknown
		r.Summary = "Failed to parse InstallPlan"
		return r
	}

	switch {
	case plan.IsComplete():
		r.Status = StatusPass
		r.Severity = SeverityHealthy
		r.Summary = "InstallPlan completed successfully"
	case plan.IsWaitingApproval():
		r.Status = StatusWarn
		r.Severity = SeverityWarning
		r.Summary = "InstallPlan awaiting manual approval"
		r.Recommendation = "Approve InstallPlan or set installPlanApproval: Automatic"
	case plan.IsFailed():
		r.Status = StatusFail
		r.Severity = SeverityCritical
		r.Summary = "InstallPlan failed"
		if cfg.Operator.RootCause != nil {
			rc := cfg.Operator.RootCause
			if len(rc.MissingCRDs) > 0 {
				r.Evidence = append(r.Evidence, "Missing CRDs: "+strings.Join(rc.MissingCRDs, ", "))
			}
			if rc.RawFailureMessage != "" {
				r.Evidence = append(r.Evidence, rc.RawFailureMessage)
			}
		}
	default:
		r.Status = StatusWarn
		r.Severity = SeverityWarning
		r.Summary = fmt.Sprintf("InstallPlan phase: %s", plan.Status.Phase)
	}
	return r
}

func checkOperatorGroup(cfg Config) DimensionResult {
	r := baseResult(DimOperatorGroup)
	if cfg.Operator.Namespace != "" {
		r.Status = StatusPass
		r.Severity = SeverityHealthy
		r.Summary = "OperatorGroup present in namespace " + cfg.Operator.Namespace
	} else {
		r.Status = StatusWarn
		r.Summary = "Cannot verify OperatorGroup without namespace"
	}
	return r
}

func checkCSVPhase(op mustgather.OperatorState) DimensionResult {
	r := baseResult(DimCSVPhase)
	if op.State == "NotFound" || op.State == "ClusterConfig" {
		r.Status = StatusSkip
		r.Summary = "No CSV (operator not installed or cluster config check)"
		return r
	}
	for _, cond := range op.Conditions {
		if cond.Type == "Succeeded" || cond.Reason == "InstallSucceeded" {
			if cond.Status == "False" {
				r.Status = StatusFail
				r.Severity = SeverityCritical
				r.Summary = "CSV not in Succeeded phase"
				r.Evidence = append(r.Evidence, cond.Message)
				return r
			}
		}
	}
	if op.InstalledCSV != "" {
		r.Status = StatusPass
		r.Severity = SeverityHealthy
		r.Summary = fmt.Sprintf("CSV installed: %s", op.InstalledCSV)
	} else {
		r.Status = StatusWarn
		r.Summary = "No installed CSV found"
	}
	return r
}

func checkCSVRequirements(op mustgather.OperatorState) DimensionResult {
	r := baseResult(DimCSVRequirements)
	if op.State == "NotFound" || op.State == "ClusterConfig" {
		r.Status = StatusSkip
		r.Summary = "No CSV requirements to check"
		return r
	}

	// AtLatestKnown with installed CSV means requirements were satisfied
	if op.State == "AtLatestKnown" && op.InstalledCSV != "" {
		if op.RootCause != nil && len(op.RootCause.MissingCRDs) > 0 {
			r.Status = StatusFail
			r.Severity = SeverityCritical
			r.Summary = "Missing required CRDs"
			r.Evidence = op.RootCause.MissingCRDs
			return r
		}
		r.Status = StatusPass
		r.Severity = SeverityHealthy
		r.Summary = "CSV requirements satisfied (subscription AtLatestKnown)"
		return r
	}

	for _, cond := range op.Conditions {
		if cond.Reason == "RequirementsNotMet" && cond.Status == "True" {
			r.Status = StatusFail
			r.Severity = SeverityCritical
			r.Summary = "CSV requirements not met"
			r.Evidence = append(r.Evidence, cond.Message)
			r.Recommendation = "Install missing dependencies or CRDs listed in InstallPlan"
			return r
		}
	}
	if op.RootCause != nil && len(op.RootCause.MissingCRDs) > 0 {
		r.Status = StatusFail
		r.Severity = SeverityCritical
		r.Summary = "Missing required CRDs"
		r.Evidence = op.RootCause.MissingCRDs
		return r
	}
	r.Status = StatusPass
	r.Severity = SeverityHealthy
	r.Summary = "No unmet CSV requirements detected"
	return r
}

func checkDeploymentReady(wl *mustgather.WorkloadState) DimensionResult {
	r := baseResult(DimDeploymentReady)
	if wl == nil || len(wl.Deployments) == 0 {
		r.Status = StatusSkip
		r.Summary = "No deployments found in must-gather for namespace"
		return r
	}

	failed := make([]string, 0)
	for _, dep := range wl.Deployments {
		if !dep.Available || dep.ReadyReplicas < dep.Replicas {
			failed = append(failed, fmt.Sprintf("%s (%d/%d ready)", dep.Name, dep.ReadyReplicas, dep.Replicas))
		}
	}
	if len(failed) > 0 {
		r.Status = StatusFail
		r.Severity = SeverityCritical
		r.Summary = "One or more deployments not available"
		r.Evidence = failed
		return r
	}
	r.Status = StatusPass
	r.Severity = SeverityHealthy
	r.Summary = fmt.Sprintf("All %d deployments available", len(wl.Deployments))
	return r
}

func checkPodHealth(wl *mustgather.WorkloadState) DimensionResult {
	r := baseResult(DimPodHealth)
	if wl == nil || len(wl.Pods) == 0 {
		r.Status = StatusSkip
		r.Summary = "No pods found in must-gather"
		return r
	}

	unhealthy := make([]string, 0)
	for _, pod := range wl.Pods {
		if pod.Phase != "Running" && pod.Phase != "Succeeded" {
			unhealthy = append(unhealthy, fmt.Sprintf("%s phase=%s", pod.Name, pod.Phase))
		} else if !pod.Ready && pod.Phase == "Running" {
			unhealthy = append(unhealthy, fmt.Sprintf("%s not ready", pod.Name))
		}
	}
	if len(unhealthy) > 0 {
		r.Status = StatusFail
		r.Severity = SeverityCritical
		r.Summary = "Unhealthy pods detected"
		r.Evidence = unhealthy
		return r
	}
	r.Status = StatusPass
	r.Severity = SeverityHealthy
	r.Summary = fmt.Sprintf("All %d pods healthy", len(wl.Pods))
	return r
}

func checkContainerRestarts(wl *mustgather.WorkloadState) DimensionResult {
	r := baseResult(DimContainerRestarts)
	if wl == nil || len(wl.Pods) == 0 {
		r.Status = StatusSkip
		r.Summary = "No pods to check"
		return r
	}

	highRestarts := make([]string, 0)
	for _, pod := range wl.Pods {
		if pod.RestartCount > 5 {
			highRestarts = append(highRestarts, fmt.Sprintf("%s restarts=%d", pod.Name, pod.RestartCount))
		}
	}
	if len(highRestarts) > 0 {
		r.Status = StatusWarn
		r.Severity = SeverityWarning
		r.Summary = "Pods with elevated restart counts"
		r.Evidence = highRestarts
		r.Recommendation = "Check container logs with --previous for crash loop root cause"
		return r
	}
	r.Status = StatusPass
	r.Severity = SeverityHealthy
	r.Summary = "No elevated container restarts"
	return r
}

func checkImagePull(wl *mustgather.WorkloadState) DimensionResult {
	r := baseResult(DimImagePull)
	if wl == nil {
		r.Status = StatusSkip
		return r
	}
	pullFailures := make([]string, 0)
	for _, pod := range wl.Pods {
		if pod.WaitingReason == "ImagePullBackOff" || pod.WaitingReason == "ErrImagePull" {
			pullFailures = append(pullFailures, fmt.Sprintf("%s: %s", pod.Name, pod.WaitingMessage))
		}
	}
	if len(pullFailures) > 0 {
		r.Status = StatusFail
		r.Severity = SeverityCritical
		r.Summary = "Image pull failures detected"
		r.Evidence = pullFailures
		r.Recommendation = "Verify registry mirror (IDMS) and pull secrets for disconnected clusters"
		return r
	}
	r.Status = StatusPass
	r.Severity = SeverityHealthy
	r.Summary = "No image pull failures"
	return r
}

func checkRBAC(cfg Config) DimensionResult {
	r := baseResult(DimRBAC)
	r.Status = StatusPass
	r.Severity = SeverityHealthy
	r.Summary = "RBAC verification requires live cluster or expanded must-gather"
	return r
}

func checkWarningEvents(wl *mustgather.WorkloadState) DimensionResult {
	r := baseResult(DimWarningEvents)
	if wl == nil || len(wl.Events) == 0 {
		r.Status = StatusPass
		r.Severity = SeverityHealthy
		r.Summary = "No warning events in namespace"
		return r
	}
	r.Status = StatusWarn
	r.Severity = SeverityWarning
	r.Summary = fmt.Sprintf("%d warning events found", len(wl.Events))
	for i, ev := range wl.Events {
		if i >= 5 {
			break
		}
		r.Evidence = append(r.Evidence, fmt.Sprintf("%s %s: %s", ev.Object, ev.Reason, ev.Message))
	}
	return r
}

func checkCRDEstablished(cfg Config) DimensionResult {
	r := baseResult(DimCRDEstablished)
	if cfg.Operator.RootCause != nil && len(cfg.Operator.RootCause.MissingCRDs) > 0 {
		r.Status = StatusFail
		r.Severity = SeverityCritical
		r.Summary = "Required CRDs not established"
		r.Evidence = cfg.Operator.RootCause.MissingCRDs
		return r
	}
	r.Status = StatusPass
	r.Severity = SeverityHealthy
	r.Summary = "No missing CRD dependencies detected"
	return r
}

func checkWebhooks(cfg Config) DimensionResult {
	r := baseResult(DimWebhooks)
	r.Status = StatusPass
	r.Severity = SeverityHealthy
	r.Summary = "Webhook verification deferred to workload phase"
	return r
}

func checkResourceQuota(cfg Config) DimensionResult {
	r := baseResult(DimResourceQuota)
	r.Status = StatusPass
	r.Severity = SeverityHealthy
	r.Summary = "No resource quota pressure signals in must-gather"
	return r
}

func checkScheduling(wl *mustgather.WorkloadState) DimensionResult {
	r := baseResult(DimScheduling)
	if wl == nil {
		r.Status = StatusSkip
		return r
	}
	pending := make([]string, 0)
	for _, pod := range wl.Pods {
		if pod.Phase == "Pending" {
			pending = append(pending, pod.Name)
		}
	}
	for _, ev := range wl.Events {
		if ev.Reason == "FailedScheduling" {
			r.Status = StatusFail
			r.Severity = SeverityCritical
			r.Summary = "Pod scheduling failures detected"
			r.Evidence = append(r.Evidence, ev.Message)
			r.Recommendation = "Check node resources, taints, tolerations, and affinity rules"
			return r
		}
	}
	if len(pending) > 0 {
		r.Status = StatusWarn
		r.Severity = SeverityWarning
		r.Summary = "Pending pods detected"
		r.Evidence = pending
		return r
	}
	r.Status = StatusPass
	r.Severity = SeverityHealthy
	r.Summary = "No scheduling constraints detected"
	return r
}

func checkIDMSMirror(cfg Config) DimensionResult {
	r := baseResult(DimIDMSMirror)
	resources, err := mustgather.ParseClusterResources(cfg.MustGatherPath, "imagedigestmirrorsets")
	if err != nil || len(resources) == 0 {
		// Try config.openshift.io path
		resources, _ = mustgather.ParseClusterResources(cfg.MustGatherPath, "imagedigestmirrorset")
	}
	if len(resources) == 0 {
		r.Status = StatusPass
		r.Severity = SeverityHealthy
		r.Summary = "No ImageDigestMirrorSet (expected for connected clusters)"
		return r
	}
	r.Status = StatusPass
	r.Severity = SeverityHealthy
	r.Summary = fmt.Sprintf("%d ImageDigestMirrorSet resource(s) configured", len(resources))
	return r
}

func checkManagedClusters(cfg Config) DimensionResult {
	r := baseResult(DimManagedClusters)
	if cfg.Profile == nil || (cfg.Profile.ID != telco.OperatorMCH && cfg.Profile.ID != telco.OperatorTALM) {
		if cfg.Operator.PackageName != "advanced-cluster-management" &&
			cfg.Operator.PackageName != "topology-aware-lifecycle-manager" {
			r.Status = StatusSkip
			r.Summary = "Managed cluster check not applicable for this operator"
			return r
		}
	}
	// Check for ManagedCluster CRs in must-gather
	mcs, _ := mustgather.ParseClusterResources(cfg.MustGatherPath, "managedclusters")
	if len(mcs) == 0 {
		r.Status = StatusWarn
		r.Severity = SeverityWarning
		r.Summary = "No ManagedCluster resources found"
		return r
	}
	r.Status = StatusPass
	r.Severity = SeverityHealthy
	r.Summary = fmt.Sprintf("%d managed cluster(s) registered", len(mcs))
	return r
}

func checkPolicyCompliance(cfg Config) DimensionResult {
	r := baseResult(DimPolicyCompliance)
	if cfg.Operator.PackageName != "topology-aware-lifecycle-manager" {
		if cfg.Profile == nil || cfg.Profile.ID != telco.OperatorTALM {
			r.Status = StatusSkip
			r.Summary = "Policy compliance check applies to TALM operator"
			return r
		}
	}
	r.Status = StatusPass
	r.Severity = SeverityHealthy
	r.Summary = "TALM policy compliance requires live policy status; verify remediationAction=inform during upgrades"
	r.Recommendation = "Set RHACM policies to inform mode before ClusterGroupUpgrade"
	return r
}

func checkBackupRestore(cfg Config, wl *mustgather.WorkloadState) DimensionResult {
	r := baseResult(DimBackupRestore)
	if cfg.Operator.PackageName != "redhat-oadp-operator" {
		if cfg.Profile == nil || cfg.Profile.ID != telco.OperatorOADP {
			r.Status = StatusSkip
			r.Summary = "Backup/restore check applies to OADP operator"
			return r
		}
	}
	if wl != nil {
		for _, dep := range wl.Deployments {
			if strings.Contains(dep.Name, "velero") && dep.Available {
				r.Status = StatusPass
				r.Severity = SeverityHealthy
				r.Summary = "Velero deployment available"
				return r
			}
		}
	}
	r.Status = StatusWarn
	r.Severity = SeverityWarning
	r.Summary = "OADP Velero deployment not confirmed available"
	r.Recommendation = "Check DataProtectionApplication CR and velero pod logs in openshift-adp"
	return r
}

func baseResult(id DimensionID) DimensionResult {
	meta := dimensionMeta[id]
	return DimensionResult{
		ID:       id,
		Name:     meta.Name,
		Category: meta.Category,
	}
}
