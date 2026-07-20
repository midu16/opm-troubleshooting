package noise

import (
	"testing"

	"github.com/midu16/opm-troubleshooting/internal/healthcheck"
)

func TestFilterKVMNoise(t *testing.T) {
	dims := []healthcheck.DimensionResult{
		{
			ID:       healthcheck.DimPodHealth,
			Name:     "Pod Phase Health",
			Status:   healthcheck.StatusFail,
			Severity: healthcheck.SeverityCritical,
			Summary:  "Unhealthy pods: node has no route to host via kvm",
		},
		{
			ID:       healthcheck.DimCSVPhase,
			Name:     "CSV Phase",
			Status:   healthcheck.StatusFail,
			Severity: healthcheck.SeverityCritical,
			Summary:  "CSV requirements not met: missing CRD",
		},
	}

	report := Filter(EnvKVM, dims)
	if report.CosmeticAlerts != 1 {
		t.Errorf("cosmetic alerts: got %d want 1", report.CosmeticAlerts)
	}
	if report.RealIssues != 1 {
		t.Errorf("real issues: got %d want 1", report.RealIssues)
	}
}

func TestFilterDisconnectedMirror(t *testing.T) {
	dims := []healthcheck.DimensionResult{
		{
			ID:       healthcheck.DimImagePull,
			Name:     "Image Pull",
			Status:   healthcheck.StatusFail,
			Severity: healthcheck.SeverityCritical,
			Summary:  "ImagePullBackOff pulling from registry.redhat.io",
		},
	}

	report := Filter(EnvDisconnected, dims)
	if report.CosmeticAlerts != 1 {
		t.Errorf("expected cosmetic for disconnected mirror pull, got real=%d cosmetic=%d",
			report.RealIssues, report.CosmeticAlerts)
	}
}

func TestParseEnvironment(t *testing.T) {
	cases := map[string]Environment{
		"lab":          EnvLab,
		"disconnected": EnvDisconnected,
		"kvm":          EnvKVM,
		"production":   EnvProduction,
		"":             EnvProduction,
	}
	for input, want := range cases {
		if got := ParseEnvironment(input); got != want {
			t.Errorf("ParseEnvironment(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFilterSkipsHealthy(t *testing.T) {
	dims := []healthcheck.DimensionResult{
		{Status: healthcheck.StatusPass, Summary: "healthy"},
		{Status: healthcheck.StatusSkip, Summary: "skipped"},
	}
	report := Filter(EnvProduction, dims)
	if report.TotalFindings != 0 {
		t.Errorf("expected 0 findings for healthy dims, got %d", report.TotalFindings)
	}
}
