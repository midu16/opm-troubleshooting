package datasource

import (
	"os"
	"testing"
	"time"

	"github.com/midu16/opm-troubleshooting/internal/mustgather"
)

// TestMustGatherSourceType verifies that SourceType returns "must-gather".
func TestMustGatherSourceType(t *testing.T) {
	src := NewMustGatherSource("/nonexistent")
	got := src.SourceType()
	if got != "must-gather" {
		t.Errorf("SourceType() = %q, want %q", got, "must-gather")
	}
}

// TestMustGatherSourceEmptyDir verifies that all getters return empty slices
// (or nil-ish results) without errors when pointed at an empty directory.
func TestMustGatherSourceEmptyDir(t *testing.T) {
	dir, err := os.MkdirTemp("", "datasource-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	src := NewMustGatherSource(dir)

	// GetSubscriptions calls ParseMustGather which returns an error when no
	// subscriptions exist, so we expect an error on an empty directory.
	subs, err := src.GetSubscriptions("test-ns")
	if err == nil {
		t.Errorf("GetSubscriptions: expected error for empty dir, got nil")
	}
	if len(subs) != 0 {
		t.Errorf("GetSubscriptions: got %d items, want 0", len(subs))
	}

	csvs, err := src.GetCSVs("test-ns")
	if err != nil {
		t.Errorf("GetCSVs: unexpected error: %v", err)
	}
	if len(csvs) != 0 {
		t.Errorf("GetCSVs: got %d items, want 0", len(csvs))
	}

	plans, err := src.GetInstallPlans("test-ns")
	if err != nil {
		t.Errorf("GetInstallPlans: unexpected error: %v", err)
	}
	if len(plans) != 0 {
		t.Errorf("GetInstallPlans: got %d items, want 0", len(plans))
	}

	catalogs, err := src.GetCatalogSources("test-ns")
	if err != nil {
		t.Errorf("GetCatalogSources: unexpected error: %v", err)
	}
	if len(catalogs) != 0 {
		t.Errorf("GetCatalogSources: got %d items, want 0", len(catalogs))
	}

	groups, err := src.GetOperatorGroups("test-ns")
	if err != nil {
		t.Errorf("GetOperatorGroups: unexpected error: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("GetOperatorGroups: got %d items, want 0", len(groups))
	}

	deps, err := src.GetDeployments("test-ns")
	if err != nil {
		t.Errorf("GetDeployments: unexpected error: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("GetDeployments: got %d items, want 0", len(deps))
	}

	pods, err := src.GetPods("test-ns")
	if err != nil {
		t.Errorf("GetPods: unexpected error: %v", err)
	}
	if len(pods) != 0 {
		t.Errorf("GetPods: got %d items, want 0", len(pods))
	}

	events, err := src.GetEvents("test-ns")
	if err != nil {
		t.Errorf("GetEvents: unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("GetEvents: got %d items, want 0", len(events))
	}

	nodes, err := src.GetNodes()
	if err != nil {
		t.Errorf("GetNodes: unexpected error: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("GetNodes: got %d items, want 0", len(nodes))
	}

	namespaces, err := src.GetNamespaces()
	if err != nil {
		t.Errorf("GetNamespaces: unexpected error: %v", err)
	}
	if len(namespaces) != 0 {
		t.Errorf("GetNamespaces: got %d items, want 0", len(namespaces))
	}

	cos, err := src.GetClusterOperators()
	if err != nil {
		t.Errorf("GetClusterOperators: unexpected error: %v", err)
	}
	if len(cos) != 0 {
		t.Errorf("GetClusterOperators: got %d items, want 0", len(cos))
	}

	routes, err := src.GetRoutes("test-ns")
	if err != nil {
		t.Errorf("GetRoutes: unexpected error: %v", err)
	}
	if len(routes) != 0 {
		t.Errorf("GetRoutes: got %d items, want 0", len(routes))
	}

	pvs, err := src.GetPVs()
	if err != nil {
		t.Errorf("GetPVs: unexpected error: %v", err)
	}
	if len(pvs) != 0 {
		t.Errorf("GetPVs: got %d items, want 0", len(pvs))
	}

	pvcs, err := src.GetPVCs("test-ns")
	if err != nil {
		t.Errorf("GetPVCs: unexpected error: %v", err)
	}
	if len(pvcs) != 0 {
		t.Errorf("GetPVCs: got %d items, want 0", len(pvcs))
	}

	scs, err := src.GetStorageClasses()
	if err != nil {
		t.Errorf("GetStorageClasses: unexpected error: %v", err)
	}
	if len(scs) != 0 {
		t.Errorf("GetStorageClasses: got %d items, want 0", len(scs))
	}

	certs, err := src.GetCertificates()
	if err != nil {
		t.Errorf("GetCertificates: unexpected error: %v", err)
	}
	if len(certs) != 0 {
		t.Errorf("GetCertificates: got %d items, want 0", len(certs))
	}

	etcd, err := src.GetEtcdMembers()
	if err != nil {
		t.Errorf("GetEtcdMembers: unexpected error: %v", err)
	}
	if len(etcd) != 0 {
		t.Errorf("GetEtcdMembers: got %d items, want 0", len(etcd))
	}
}

// TestConvertOperatorRoundTrip converts an OperatorState to mustgather.OperatorState
// via OperatorToMustGather and back via OperatorFromMustGather, verifying all fields
// are preserved.
func TestConvertOperatorRoundTrip(t *testing.T) {
	now := time.Date(2025, 3, 15, 10, 30, 0, 0, time.UTC)

	original := OperatorState{
		PackageName:      "my-operator",
		Namespace:        "openshift-operators",
		Channel:          "stable-v1",
		InstalledCSV:     "my-operator.v1.2.3",
		CurrentCSV:       "my-operator.v1.2.3",
		InstalledVersion: "1.2.3",
		State:            "AtLatestKnown",
		InstallPlanRef:   "install-abc123",
		Faulty:           true,
		FailureReason:    "missing CRD",
		Conditions: []Condition{
			{
				Type:               "CatalogSourcesUnhealthy",
				Status:             "False",
				Reason:             "AllCatalogSourcesHealthy",
				Message:            "all catalog sources are healthy",
				LastTransitionTime: now,
			},
		},
		RootCause: &RootCauseDetail{
			MissingCRDs:         []string{"widgets.example.com"},
			MissingAPIs:         []string{"v1beta1.widgets.example.com"},
			UnknownResources:    []string{"unknown-res"},
			NotPresentResources: []string{"not-present-res"},
			PodErrors:           []string{"CrashLoopBackOff"},
			RawFailureMessage:   "install failed: missing CRD",
		},
	}

	// Round-trip: datasource -> mustgather -> datasource
	mgOp := OperatorToMustGather(original)
	roundTripped := OperatorFromMustGather(mgOp)

	// Verify scalar fields
	if roundTripped.PackageName != original.PackageName {
		t.Errorf("PackageName = %q, want %q", roundTripped.PackageName, original.PackageName)
	}
	if roundTripped.Namespace != original.Namespace {
		t.Errorf("Namespace = %q, want %q", roundTripped.Namespace, original.Namespace)
	}
	if roundTripped.Channel != original.Channel {
		t.Errorf("Channel = %q, want %q", roundTripped.Channel, original.Channel)
	}
	if roundTripped.InstalledCSV != original.InstalledCSV {
		t.Errorf("InstalledCSV = %q, want %q", roundTripped.InstalledCSV, original.InstalledCSV)
	}
	if roundTripped.CurrentCSV != original.CurrentCSV {
		t.Errorf("CurrentCSV = %q, want %q", roundTripped.CurrentCSV, original.CurrentCSV)
	}
	if roundTripped.InstalledVersion != original.InstalledVersion {
		t.Errorf("InstalledVersion = %q, want %q", roundTripped.InstalledVersion, original.InstalledVersion)
	}
	if roundTripped.State != original.State {
		t.Errorf("State = %q, want %q", roundTripped.State, original.State)
	}
	if roundTripped.InstallPlanRef != original.InstallPlanRef {
		t.Errorf("InstallPlanRef = %q, want %q", roundTripped.InstallPlanRef, original.InstallPlanRef)
	}
	if roundTripped.Faulty != original.Faulty {
		t.Errorf("Faulty = %v, want %v", roundTripped.Faulty, original.Faulty)
	}
	if roundTripped.FailureReason != original.FailureReason {
		t.Errorf("FailureReason = %q, want %q", roundTripped.FailureReason, original.FailureReason)
	}

	// Verify conditions
	if len(roundTripped.Conditions) != len(original.Conditions) {
		t.Fatalf("Conditions length = %d, want %d", len(roundTripped.Conditions), len(original.Conditions))
	}
	c := roundTripped.Conditions[0]
	if c.Type != "CatalogSourcesUnhealthy" {
		t.Errorf("Condition.Type = %q, want %q", c.Type, "CatalogSourcesUnhealthy")
	}
	if c.Status != "False" {
		t.Errorf("Condition.Status = %q, want %q", c.Status, "False")
	}
	if c.Reason != "AllCatalogSourcesHealthy" {
		t.Errorf("Condition.Reason = %q, want %q", c.Reason, "AllCatalogSourcesHealthy")
	}
	if c.Message != "all catalog sources are healthy" {
		t.Errorf("Condition.Message = %q, want %q", c.Message, "all catalog sources are healthy")
	}
	if !c.LastTransitionTime.Equal(now) {
		t.Errorf("Condition.LastTransitionTime = %v, want %v", c.LastTransitionTime, now)
	}

	// Verify RootCause
	if roundTripped.RootCause == nil {
		t.Fatal("RootCause is nil after round-trip")
	}
	rc := roundTripped.RootCause
	assertStringSlice(t, "MissingCRDs", rc.MissingCRDs, original.RootCause.MissingCRDs)
	assertStringSlice(t, "MissingAPIs", rc.MissingAPIs, original.RootCause.MissingAPIs)
	assertStringSlice(t, "UnknownResources", rc.UnknownResources, original.RootCause.UnknownResources)
	assertStringSlice(t, "NotPresentResources", rc.NotPresentResources, original.RootCause.NotPresentResources)
	assertStringSlice(t, "PodErrors", rc.PodErrors, original.RootCause.PodErrors)
	if rc.RawFailureMessage != original.RootCause.RawFailureMessage {
		t.Errorf("RootCause.RawFailureMessage = %q, want %q", rc.RawFailureMessage, original.RootCause.RawFailureMessage)
	}
}

// TestConvertDeploymentFromMustGather verifies DeploymentFromMustGather maps all fields.
func TestConvertDeploymentFromMustGather(t *testing.T) {
	mgDep := mustgather.DeploymentState{
		Name:           "controller-manager",
		Replicas:       2,
		ReadyReplicas:  1,
		Available:      false,
		Progressing:    true,
		ProgressingMsg: "waiting for rollout",
		UnavailableMsg: "1 replica unavailable",
	}

	ds := DeploymentFromMustGather(mgDep)

	if ds.Name != "controller-manager" {
		t.Errorf("Name = %q, want %q", ds.Name, "controller-manager")
	}
	if ds.Replicas != 2 {
		t.Errorf("Replicas = %d, want %d", ds.Replicas, 2)
	}
	if ds.ReadyReplicas != 1 {
		t.Errorf("ReadyReplicas = %d, want %d", ds.ReadyReplicas, 1)
	}
	if ds.Available != false {
		t.Errorf("Available = %v, want false", ds.Available)
	}
	if ds.Progressing != true {
		t.Errorf("Progressing = %v, want true", ds.Progressing)
	}
	if ds.ProgressingMsg != "waiting for rollout" {
		t.Errorf("ProgressingMsg = %q, want %q", ds.ProgressingMsg, "waiting for rollout")
	}
	if ds.UnavailableMsg != "1 replica unavailable" {
		t.Errorf("UnavailableMsg = %q, want %q", ds.UnavailableMsg, "1 replica unavailable")
	}
}

// TestConvertPodFromMustGather verifies PodFromMustGather maps all fields.
func TestConvertPodFromMustGather(t *testing.T) {
	mgPod := mustgather.PodState{
		Name:             "operator-pod-abc123",
		Phase:            "Running",
		Ready:            true,
		RestartCount:     5,
		WaitingReason:    "CrashLoopBackOff",
		WaitingMessage:   "back-off 5m0s restarting failed container",
		TerminatedReason: "OOMKilled",
	}

	ds := PodFromMustGather(mgPod)

	if ds.Name != "operator-pod-abc123" {
		t.Errorf("Name = %q, want %q", ds.Name, "operator-pod-abc123")
	}
	if ds.Phase != "Running" {
		t.Errorf("Phase = %q, want %q", ds.Phase, "Running")
	}
	if ds.Ready != true {
		t.Errorf("Ready = %v, want true", ds.Ready)
	}
	if ds.RestartCount != 5 {
		t.Errorf("RestartCount = %d, want %d", ds.RestartCount, 5)
	}
	if ds.WaitingReason != "CrashLoopBackOff" {
		t.Errorf("WaitingReason = %q, want %q", ds.WaitingReason, "CrashLoopBackOff")
	}
	if ds.WaitingMessage != "back-off 5m0s restarting failed container" {
		t.Errorf("WaitingMessage = %q, want %q", ds.WaitingMessage, "back-off 5m0s restarting failed container")
	}
	if ds.TerminatedReason != "OOMKilled" {
		t.Errorf("TerminatedReason = %q, want %q", ds.TerminatedReason, "OOMKilled")
	}
}

// TestConvertEventFromMustGather verifies EventFromMustGather maps all fields.
func TestConvertEventFromMustGather(t *testing.T) {
	mgEvent := mustgather.EventState{
		Type:    "Warning",
		Reason:  "FailedScheduling",
		Message: "0/6 nodes are available: insufficient cpu",
		Object:  "Pod/my-pod-abc123",
	}

	ds := EventFromMustGather(mgEvent)

	if ds.Type != "Warning" {
		t.Errorf("Type = %q, want %q", ds.Type, "Warning")
	}
	if ds.Reason != "FailedScheduling" {
		t.Errorf("Reason = %q, want %q", ds.Reason, "FailedScheduling")
	}
	if ds.Message != "0/6 nodes are available: insufficient cpu" {
		t.Errorf("Message = %q, want %q", ds.Message, "0/6 nodes are available: insufficient cpu")
	}
	if ds.Object != "Pod/my-pod-abc123" {
		t.Errorf("Object = %q, want %q", ds.Object, "Pod/my-pod-abc123")
	}
}

// assertStringSlice is a test helper that compares two string slices.
func assertStringSlice(t *testing.T, name string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s length = %d, want %d", name, len(got), len(want))
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("%s[%d] = %q, want %q", name, i, got[i], want[i])
		}
	}
}
