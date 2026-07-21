package healthcheck

import (
	"context"
	"testing"

	"github.com/midu16/opm-troubleshooting/internal/mustgather"
)

func TestAllDimensionIDs(t *testing.T) {
	olmIDs := OLMDimensionIDs()
	if len(olmIDs) != 20 {
		t.Fatalf("expected 20 OLM dimensions, got %d", len(olmIDs))
	}
	infraIDs := InfraDimensionIDs()
	if len(infraIDs) != 13 {
		t.Fatalf("expected 13 infra dimensions, got %d", len(infraIDs))
	}
	allIDs := AllDimensionIDs()
	if len(allIDs) != 33 {
		t.Fatalf("expected 33 total dimensions, got %d", len(allIDs))
	}
}

func TestCheckSubscriptionHealthy(t *testing.T) {
	r := checkSubscription(mustgather.OperatorState{State: "AtLatestKnown"})
	if r.Status != StatusPass {
		t.Errorf("expected pass, got %s", r.Status)
	}
}

func TestCheckSubscriptionFailed(t *testing.T) {
	r := checkSubscription(mustgather.OperatorState{State: "Failed"})
	if r.Status != StatusFail {
		t.Errorf("expected fail, got %s", r.Status)
	}
}

func TestRunMinimal(t *testing.T) {
	ctx := context.Background()
	report, err := Run(ctx, Config{
		MustGatherPath: t.TempDir(),
		Operator: mustgather.OperatorState{
			PackageName:  "redhat-oadp-operator",
			Namespace:    "openshift-adp",
			State:        "AtLatestKnown",
			InstalledCSV: "redhat-oadp-operator.v1.0.0",
		},
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	olmCount := len(OLMDimensionIDs())
	if report.TotalDimensions != olmCount {
		t.Errorf("total dimensions: got %d want %d", report.TotalDimensions, olmCount)
	}
	if len(report.Dimensions) != olmCount {
		t.Errorf("dimension results: got %d want %d", len(report.Dimensions), olmCount)
	}
}
