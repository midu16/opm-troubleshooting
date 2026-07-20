package openshift

import (
	"testing"

	"github.com/midu16/opm-troubleshooting/internal/codeanalysis"
	"github.com/midu16/opm-troubleshooting/internal/healthcheck"
)

func TestClassifyCodeBug(t *testing.T) {
	matches := []codeanalysis.Match{
		{
			FilePath:    "pkg/controller/reconcile.go",
			LineNumber:  42,
			LineContent: `return fmt.Errorf("failed to reconcile resource")`,
			Pattern:     "failed to reconcile",
		},
	}

	result := Classify(matches, "failed to reconcile resource", nil)

	if result.Type != ClassCodeBug {
		t.Errorf("Type = %q, want %q", result.Type, ClassCodeBug)
	}
	if result.Confidence != 0.8 {
		t.Errorf("Confidence = %f, want 0.8", result.Confidence)
	}
	if len(result.Evidence) < 1 {
		t.Fatal("expected at least 1 evidence entry")
	}
}

func TestClassifyConfiguration(t *testing.T) {
	result := Classify(nil, "RBAC forbidden: user cannot list pods", nil)

	if result.Type != ClassConfiguration {
		t.Errorf("Type = %q, want %q", result.Type, ClassConfiguration)
	}
	if result.Confidence != 0.7 {
		t.Errorf("Confidence = %f, want 0.7", result.Confidence)
	}
}

func TestClassifyInfrastructure(t *testing.T) {
	tests := []struct {
		name          string
		failureReason string
		infraReport   *healthcheck.Report
	}{
		{
			name:          "infra keywords in failure reason",
			failureReason: "node not ready, pod evicted",
			infraReport:   nil,
		},
		{
			name:          "infra report with failures",
			failureReason: "some generic error",
			infraReport: &healthcheck.Report{
				Failed: 1,
				Dimensions: []healthcheck.DimensionResult{
					{
						Name:     "Node Health",
						Status:   healthcheck.StatusFail,
						Severity: healthcheck.SeverityCritical,
						Summary:  "2 nodes NotReady",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Classify(nil, tt.failureReason, tt.infraReport)

			if result.Type != ClassInfrastructure {
				t.Errorf("Type = %q, want %q", result.Type, ClassInfrastructure)
			}
			if result.Confidence != 0.7 {
				t.Errorf("Confidence = %f, want 0.7", result.Confidence)
			}
		})
	}
}

func TestClassifyUnknown(t *testing.T) {
	result := Classify(nil, "", nil)

	if result.Type != ClassUnknown {
		t.Errorf("Type = %q, want %q", result.Type, ClassUnknown)
	}
}

func TestClassifyCodeBugWithConfigOverlap(t *testing.T) {
	matches := []codeanalysis.Match{
		{
			FilePath:    "pkg/auth/rbac.go",
			LineNumber:  15,
			LineContent: `log.Error("forbidden: service account lacks permission")`,
			Pattern:     "forbidden",
		},
	}

	// "forbidden" is both a code match and a config keyword
	result := Classify(matches, "forbidden access to resource", nil)

	if result.Type != ClassCodeBug {
		t.Errorf("Type = %q, want %q (code match should still win)", result.Type, ClassCodeBug)
	}
	if result.Confidence != 0.6 {
		t.Errorf("Confidence = %f, want 0.6 (reduced due to config overlap)", result.Confidence)
	}

	// Check that overlap evidence is noted
	foundOverlapEvidence := false
	for _, e := range result.Evidence {
		if e == "Error also relates to configuration — could be validation code rather than a bug" {
			foundOverlapEvidence = true
			break
		}
	}
	if !foundOverlapEvidence {
		t.Error("expected overlap evidence note, not found in Evidence slice")
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		keywords []string
		want     bool
	}{
		{
			name:     "matching keyword",
			text:     "the node not ready event was logged",
			keywords: []string{"node not ready", "disk pressure"},
			want:     true,
		},
		{
			name:     "no match",
			text:     "everything is fine",
			keywords: []string{"error", "failure"},
			want:     false,
		},
		{
			name:     "empty text",
			text:     "",
			keywords: []string{"error"},
			want:     false,
		},
		{
			name:     "empty keywords",
			text:     "some text",
			keywords: []string{},
			want:     false,
		},
		{
			name:     "nil keywords",
			text:     "some text",
			keywords: nil,
			want:     false,
		},
		{
			name:     "partial match within word",
			text:     "unauthorized access attempt",
			keywords: []string{"unauthorized"},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsAny(tt.text, tt.keywords)
			if got != tt.want {
				t.Errorf("containsAny(%q, %v) = %v, want %v", tt.text, tt.keywords, got, tt.want)
			}
		})
	}
}
