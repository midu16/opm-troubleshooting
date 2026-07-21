package openshift

import (
	"testing"

	"github.com/midu16/opm-troubleshooting/internal/codeanalysis"
	"github.com/midu16/opm-troubleshooting/internal/gitdelta"
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

func TestClassifyEnhanced_CodeDelta(t *testing.T) {
	matches := []codeanalysis.Match{
		{
			FilePath:    "pkg/controller/reconcile.go",
			LineNumber:  42,
			LineContent: `return fmt.Errorf("failed to reconcile resource")`,
			Pattern:     "failed to reconcile",
		},
	}
	delta := &gitdelta.CommitDelta{
		FilesChanged: []string{
			"pkg/controller/reconcile.go",
			"internal/operator/handler.go",
			"cmd/manager/main.go",
		},
		Additions: 50,
		Deletions: 10,
	}

	result := ClassifyEnhanced(matches, "failed to reconcile resource", nil, delta, "abc123", true)

	if result.Type != ClassCodeBug {
		t.Errorf("Type = %q, want %q", result.Type, ClassCodeBug)
	}
	if result.Confidence <= 0.8 {
		t.Errorf("Confidence = %f, want > 0.8 with code delta + pinned commit", result.Confidence)
	}
}

func TestClassifyEnhanced_ConfigDelta(t *testing.T) {
	delta := &gitdelta.CommitDelta{
		FilesChanged: []string{
			"config/rbac/role.yaml",
			"manifests/crd.yaml",
			"deploy/operator.yaml",
		},
		DiffSummary: "Update RBAC permissions for new CRD",
	}

	result := ClassifyEnhanced(nil, "RBAC forbidden: user cannot list pods", nil, delta, "", false)

	if result.Type != ClassConfiguration {
		t.Errorf("Type = %q, want %q", result.Type, ClassConfiguration)
	}
}

func TestClassifyEnhanced_CommitPinned(t *testing.T) {
	matches := []codeanalysis.Match{
		{
			FilePath:    "pkg/controller/reconcile.go",
			LineNumber:  42,
			LineContent: `return fmt.Errorf("resource error")`,
			Pattern:     "resource error",
		},
	}

	pinned := ClassifyEnhanced(matches, "resource error", nil, nil, "abc123", true)
	unpinned := ClassifyEnhanced(matches, "resource error", nil, nil, "", false)

	if pinned.Confidence <= unpinned.Confidence {
		t.Errorf("Pinned confidence (%f) should be > unpinned (%f)", pinned.Confidence, unpinned.Confidence)
	}
}

func TestClassifyEnhanced_NilDelta(t *testing.T) {
	result := ClassifyEnhanced(nil, "RBAC forbidden: user cannot list pods", nil, nil, "", false)

	if result.Type != ClassConfiguration {
		t.Errorf("Type = %q, want %q", result.Type, ClassConfiguration)
	}
	if result.Confidence <= 0 {
		t.Error("expected non-zero confidence")
	}
}

func TestClassifyFileChanges(t *testing.T) {
	files := []string{
		"pkg/controller/reconcile.go",
		"internal/operator/handler.go",
		"config/rbac/role.yaml",
		"manifests/crd.yaml",
		"README.md",
	}

	codeFiles, configFiles := classifyFileChanges(files)

	if codeFiles != 2 {
		t.Errorf("codeFiles = %d, want 2", codeFiles)
	}
	if configFiles != 2 {
		t.Errorf("configFiles = %d, want 2", configFiles)
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
