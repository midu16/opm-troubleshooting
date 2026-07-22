package mustgather

import (
	"context"
	"os"
	"testing"
)

func TestParseMustGather(t *testing.T) {
	// This test requires a real must-gather directory
	// Skip if not available
	mustGatherPath := os.Getenv("TEST_MUST_GATHER_PATH")
	if mustGatherPath == "" {
		t.Skip("Skipping test: TEST_MUST_GATHER_PATH not set")
	}

	if _, err := os.Stat(mustGatherPath); os.IsNotExist(err) {
		t.Skipf("Must-gather path does not exist: %s", mustGatherPath)
	}

	ctx := context.Background()
	result, err := ParseMustGather(ctx, mustGatherPath)
	if err != nil {
		t.Fatalf("ParseMustGather() error = %v", err)
	}

	if len(result.Operators) == 0 {
		t.Error("Expected at least one operator, got 0")
	}

	t.Logf("Parsed %d operators, %d faulty", len(result.Operators), result.FaultyCount)

	// Validate operator data structure
	for i, op := range result.Operators {
		if op.PackageName == "" {
			t.Errorf("Operator %d has empty PackageName", i)
		}
		if op.Namespace == "" {
			t.Errorf("Operator %d (%s) has empty Namespace", i, op.PackageName)
		}
		if op.Channel == "" {
			t.Errorf("Operator %d (%s) has empty Channel", i, op.PackageName)
		}
		if op.State == "" {
			t.Errorf("Operator %d (%s) has empty State", i, op.PackageName)
		}

		if op.Faulty && op.FailureReason == "" {
			t.Errorf("Faulty operator %d (%s) has empty FailureReason", i, op.PackageName)
		}
	}
}

func TestIsFaulty(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		operator OperatorState
		want     bool
	}{
		{
			name: "Healthy operator",
			operator: OperatorState{
				State:      "AtLatestKnown",
				Conditions: []Condition{},
			},
			want: false,
		},
		{
			name: "Upgrade pending without InstallPlan",
			operator: OperatorState{
				State: "UpgradePending",
			},
			want: false, // Requires InstallPlan verification to avoid false positives
		},
		{
			name: "Failed state",
			operator: OperatorState{
				State: "Failed",
			},
			want: true,
		},
		{
			name: "CatalogSourcesUnhealthy",
			operator: OperatorState{
				State: "AtLatestKnown",
				Conditions: []Condition{
					{
						Type:   "CatalogSourcesUnhealthy",
						Status: "True",
					},
				},
			},
			want: true,
		},
		{
			name: "RequirementsNotMet without InstallPlan",
			operator: OperatorState{
				State: "AtLatestKnown",
				Conditions: []Condition{
					{
						Reason: "RequirementsNotMet",
					},
				},
			},
			want: false, // Verified via InstallPlan, not subscription condition alone
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isFaulty(ctx, &tt.operator, "")
			if got != tt.want {
				t.Errorf("isFaulty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildFailureReason(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name         string
		operator     OperatorState
		wantContains []string
	}{
		{
			name: "Upgrade pending with conditions",
			operator: OperatorState{
				State: "UpgradePending",
				Conditions: []Condition{
					{
						Type:    "CatalogSourcesUnhealthy",
						Status:  "True",
						Message: "catalog missing",
					},
				},
			},
			wantContains: []string{"Upgrade pending", "CatalogSourcesUnhealthy"},
		},
		{
			name: "Failed state",
			operator: OperatorState{
				State: "Failed",
			},
			wantContains: []string{"Subscription failed"},
		},
		{
			name: "Requirements not met",
			operator: OperatorState{
				State: "AtLatestKnown",
				Conditions: []Condition{
					{
						Reason:  "RequirementsNotMet",
						Message: "dependencies missing",
					},
				},
			},
			wantContains: []string{"RequirementsNotMet"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildFailureReason(ctx, &tt.operator, "")
			if got == "" {
				t.Error("buildFailureReason() returned empty string")
			}
			for _, substr := range tt.wantContains {
				if !contains(got, substr) {
					t.Errorf("buildFailureReason() = %q, want to contain %q", got, substr)
				}
			}
		})
	}
}

func TestExtractVersionFromCSVName(t *testing.T) {
	tests := []struct {
		csvName string
		want    string
	}{
		{"cluster-logging.v6.0.0", "6.0.0"},
		{"compliance-operator.v1.6.0", "1.6.0"},
		{"falcon-operator.v1.11.0", "1.11.0"},
		{"ocs-operator.v4.16.2-rhodf", "4.16.2-rhodf"},
		{"no-version", "no-version"},
	}

	for _, tt := range tests {
		t.Run(tt.csvName, func(t *testing.T) {
			got := extractVersionFromCSVName(tt.csvName)
			if got != tt.want {
				t.Errorf("extractVersionFromCSVName(%q) = %q, want %q", tt.csvName, got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
