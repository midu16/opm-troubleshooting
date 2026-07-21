package learning

import (
	"math"
	"testing"

	"github.com/midu16/opm-troubleshooting/internal/healthcheck"
	"github.com/midu16/opm-troubleshooting/internal/mustgather"
	"github.com/midu16/opm-troubleshooting/internal/noise"
	"github.com/midu16/opm-troubleshooting/internal/rca"
)

const floatTolerance = 1e-9

func TestBuildFingerprint(t *testing.T) {
	input := SymptomInput{
		Operator: mustgather.OperatorState{
			PackageName:   "sriov-network-operator",
			State:         "Failed",
			FailureReason: "missing CRD sriovnetworknodepolicies.sriovnetwork.openshift.io",
		},
		HealthReport: &healthcheck.Report{
			Failed: 2,
			Dimensions: []healthcheck.DimensionResult{
				{
					Name:   "CRD Established Status",
					Status: healthcheck.StatusFail,
				},
				{
					Name:   "Deployment Availability",
					Status: healthcheck.StatusFail,
				},
				{
					Name:   "Pod Phase Health",
					Status: healthcheck.StatusPass,
				},
			},
		},
		RCAPatterns: []rca.PatternMatch{
			{
				Pattern:     rca.PatternMissingGuard,
				Confidence:  0.8,
				Description: "Missing null/error checks causing crashes",
				Evidence:    []string{"nil pointer dereference"},
			},
			{
				Pattern:     rca.PatternRaceCondition,
				Confidence:  0.3, // below 0.5 threshold, should be excluded
				Description: "Timing-dependent failures",
				Evidence:    []string{"intermittent"},
			},
		},
	}

	fp := BuildFingerprint(input)

	if fp.Operator != "sriov-network-operator" {
		t.Errorf("Operator = %q, want %q", fp.Operator, "sriov-network-operator")
	}

	if fp.SymptomHash == "" {
		t.Error("SymptomHash is empty, expected a non-empty SHA256 hash")
	}

	if len(fp.Symptoms) == 0 {
		t.Fatal("Symptoms slice is empty, expected normalized symptoms")
	}

	// Verify specific symptoms are present
	wantSymptoms := map[string]bool{
		"state:failed":               false,
		"health:crd established status:fail": false,
		"health:deployment availability:fail": false,
		"pattern:missing_guard":      false,
	}
	for _, s := range fp.Symptoms {
		if _, ok := wantSymptoms[s]; ok {
			wantSymptoms[s] = true
		}
	}
	for sym, found := range wantSymptoms {
		if !found {
			t.Errorf("expected symptom %q not found in %v", sym, fp.Symptoms)
		}
	}

	// RaceCondition with confidence 0.3 should NOT appear (threshold is 0.5)
	for _, s := range fp.Symptoms {
		if s == "pattern:race_condition" {
			t.Error("pattern:race_condition should not appear (confidence 0.3 < 0.5 threshold)")
		}
	}

	// Classification should be "configuration" because of "missing CRD"
	if fp.Classification != "configuration" {
		t.Errorf("Classification = %q, want %q", fp.Classification, "configuration")
	}

	if fp.Confidence <= 0 || fp.Confidence > 1.0 {
		t.Errorf("Confidence = %f, expected value in (0, 1.0]", fp.Confidence)
	}
}

func TestNormalizeSymptoms(t *testing.T) {
	input := SymptomInput{
		Operator: mustgather.OperatorState{
			State:         "UpgradePending",
			FailureReason: "RBAC forbidden access denied for serviceaccount",
		},
		HealthReport: &healthcheck.Report{
			Failed: 1,
			Dimensions: []healthcheck.DimensionResult{
				{
					Name:   "ServiceAccount & RBAC",
					Status: healthcheck.StatusFail,
				},
				{
					Name:   "Pod Phase Health",
					Status: healthcheck.StatusPass,
				},
			},
		},
		InfraReport: &healthcheck.Report{
			Failed: 1,
			Dimensions: []healthcheck.DimensionResult{
				{
					Name:   "Node Health",
					Status: healthcheck.StatusFail,
				},
				{
					Name:   "etcd Cluster Health",
					Status: healthcheck.StatusPass,
				},
			},
		},
		RCAPatterns: []rca.PatternMatch{
			{
				Pattern:    rca.PatternStateDivergence,
				Confidence: 0.7,
			},
		},
	}

	symptoms := NormalizeSymptoms(input)

	// State should be present (not "AtLatestKnown")
	assertContains(t, symptoms, "state:upgradepending")

	// Failure keywords extracted from FailureReason (words >= 4 chars, non-common)
	assertContains(t, symptoms, "failure:rbac")
	assertContains(t, symptoms, "failure:forbidden")
	assertContains(t, symptoms, "failure:access")
	assertContains(t, symptoms, "failure:denied")
	assertContains(t, symptoms, "failure:serviceaccount")

	// Health dimension failures
	assertContains(t, symptoms, "health:serviceaccount & rbac:fail")

	// Infra dimension failures
	assertContains(t, symptoms, "infra:node health:fail")

	// RCA pattern above threshold
	assertContains(t, symptoms, "pattern:state_divergence")

	// Passing dimensions should NOT generate symptoms
	assertNotContains(t, symptoms, "health:pod phase health:fail")
	assertNotContains(t, symptoms, "infra:etcd cluster health:fail")
}

func TestBuildFingerprintEmpty(t *testing.T) {
	input := SymptomInput{
		Operator: mustgather.OperatorState{
			PackageName: "empty-operator",
			State:       "AtLatestKnown", // healthy state, excluded from symptoms
		},
	}

	fp := BuildFingerprint(input)

	if fp.Operator != "empty-operator" {
		t.Errorf("Operator = %q, want %q", fp.Operator, "empty-operator")
	}

	// SymptomHash should still be computed (hash of empty/no symptoms)
	if fp.SymptomHash == "" {
		t.Error("SymptomHash should not be empty even with no symptoms")
	}

	// Symptoms may be empty or nil
	if len(fp.Symptoms) != 0 {
		t.Errorf("expected 0 symptoms for healthy operator, got %d: %v", len(fp.Symptoms), fp.Symptoms)
	}

	// Confidence should be the base value (0.3) since no data is available
	if math.Abs(fp.Confidence-0.3) > floatTolerance {
		t.Errorf("Confidence = %f, want 0.3 (base confidence with no data)", fp.Confidence)
	}

	// Classification should be empty (no failure reason)
	if fp.Classification != "" {
		t.Errorf("Classification = %q, want empty string", fp.Classification)
	}
}

func TestBuildFingerprintClassification(t *testing.T) {
	tests := []struct {
		name           string
		failureReason  string
		wantClass      string
	}{
		{
			name:          "missing CRD triggers configuration",
			failureReason: "missing CRD for sriovnetworknodepolicies",
			wantClass:     "configuration",
		},
		{
			name:          "RBAC triggers configuration",
			failureReason: "RBAC policy denies access",
			wantClass:     "configuration",
		},
		{
			name:          "forbidden triggers configuration",
			failureReason: "forbidden: user cannot create resource",
			wantClass:     "configuration",
		},
		{
			name:          "configmap triggers configuration",
			failureReason: "configmap not found in namespace",
			wantClass:     "configuration",
		},
		{
			name:          "unrelated failure gets no classification",
			failureReason: "pod crash looping OOMKilled",
			wantClass:     "",
		},
		{
			name:          "empty failure reason gets no classification",
			failureReason: "",
			wantClass:     "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := SymptomInput{
				Operator: mustgather.OperatorState{
					PackageName:   "test-operator",
					FailureReason: tc.failureReason,
				},
			}
			fp := BuildFingerprint(input)
			if fp.Classification != tc.wantClass {
				t.Errorf("Classification = %q, want %q", fp.Classification, tc.wantClass)
			}
		})
	}
}

func TestComputeConfidence(t *testing.T) {
	// Base confidence with no data
	base := SymptomInput{
		Operator: mustgather.OperatorState{},
	}
	baseConf := BuildFingerprint(base).Confidence
	if math.Abs(baseConf-0.3) > floatTolerance {
		t.Errorf("base confidence = %f, want 0.3", baseConf)
	}

	// Adding failure reason increases confidence by 0.2
	withFailure := SymptomInput{
		Operator: mustgather.OperatorState{
			FailureReason: "something went wrong",
		},
	}
	failConf := BuildFingerprint(withFailure).Confidence
	if math.Abs(failConf-0.5) > floatTolerance {
		t.Errorf("confidence with failure reason = %f, want 0.5", failConf)
	}

	// Adding health report with failures increases by another 0.2
	withHealth := SymptomInput{
		Operator: mustgather.OperatorState{
			FailureReason: "something went wrong",
		},
		HealthReport: &healthcheck.Report{
			Failed: 1,
		},
	}
	healthConf := BuildFingerprint(withHealth).Confidence
	if math.Abs(healthConf-0.7) > floatTolerance {
		t.Errorf("confidence with failure + health = %f, want 0.7", healthConf)
	}

	// Adding RCA patterns increases by another 0.2
	withRCA := SymptomInput{
		Operator: mustgather.OperatorState{
			FailureReason: "something went wrong",
		},
		HealthReport: &healthcheck.Report{
			Failed: 1,
		},
		RCAPatterns: []rca.PatternMatch{
			{Pattern: rca.PatternMissingGuard, Confidence: 0.8},
		},
	}
	rcaConf := BuildFingerprint(withRCA).Confidence
	if math.Abs(rcaConf-0.9) > floatTolerance {
		t.Errorf("confidence with failure + health + RCA = %f, want 0.9", rcaConf)
	}

	// Adding noise report with real issues adds 0.1, capped at 1.0
	withNoise := SymptomInput{
		Operator: mustgather.OperatorState{
			FailureReason: "something went wrong",
		},
		HealthReport: &healthcheck.Report{
			Failed: 1,
		},
		RCAPatterns: []rca.PatternMatch{
			{Pattern: rca.PatternMissingGuard, Confidence: 0.8},
		},
		NoiseReport: &noise.FilterReport{
			RealIssues: 3,
		},
	}
	noiseConf := BuildFingerprint(withNoise).Confidence
	if math.Abs(noiseConf-1.0) > floatTolerance {
		t.Errorf("confidence with all data = %f, want 1.0 (capped)", noiseConf)
	}

	// Verify monotonic increase: each additional data source raises confidence
	if failConf <= baseConf {
		t.Error("adding failure reason should increase confidence")
	}
	if healthConf <= failConf {
		t.Error("adding health report should increase confidence")
	}
	if rcaConf <= healthConf {
		t.Error("adding RCA patterns should increase confidence")
	}
	if noiseConf < rcaConf {
		t.Error("adding noise report should not decrease confidence")
	}
}

// assertContains checks that the slice contains the expected string.
func assertContains(t *testing.T, slice []string, want string) {
	t.Helper()
	for _, s := range slice {
		if s == want {
			return
		}
	}
	t.Errorf("slice %v does not contain %q", slice, want)
}

// assertNotContains checks that the slice does not contain the given string.
func assertNotContains(t *testing.T, slice []string, unwanted string) {
	t.Helper()
	for _, s := range slice {
		if s == unwanted {
			t.Errorf("slice should not contain %q but it does", unwanted)
			return
		}
	}
}
