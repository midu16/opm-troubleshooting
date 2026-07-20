package learning

import (
	"testing"

	"github.com/midu16/opm-troubleshooting/internal/metadata"
)

// openTestStore creates a MetadataStore in a temporary directory for testing.
func openTestStore(t *testing.T) *metadata.MetadataStore {
	t.Helper()
	dir := t.TempDir()
	store, err := metadata.Open(dir)
	if err != nil {
		t.Fatalf("metadata.Open(%q): %v", dir, err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestFindSimilarIssuesExact(t *testing.T) {
	store := openTestStore(t)

	symptoms := []string{"state:failed", "health:crd established status:fail", "pattern:missing_guard"}
	hash := metadata.ComputeSymptomHash(symptoms)

	// Save a fingerprint into the store.
	saved := metadata.Fingerprint{
		SymptomHash:    hash,
		Operator:       "sriov-network-operator",
		Symptoms:       symptoms,
		RootCause:      "Missing CRD dependency",
		Classification: "configuration",
		Resolution:     "Install missing CRD from dependency operator",
		Confidence:     0.9,
	}
	_, err := store.SaveFingerprint(saved)
	if err != nil {
		t.Fatalf("SaveFingerprint: %v", err)
	}

	// Search with the exact same symptoms.
	query := metadata.Fingerprint{
		SymptomHash: hash,
		Operator:    "sriov-network-operator",
		Symptoms:    symptoms,
	}

	results, err := FindSimilarIssues(store, query)
	if err != nil {
		t.Fatalf("FindSimilarIssues: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least 1 result for exact match, got 0")
	}

	// First result should be the exact match with similarity 1.0
	exact := results[0]
	if exact.Similarity != 1.0 {
		t.Errorf("exact match Similarity = %f, want 1.0", exact.Similarity)
	}
	if exact.SymptomHash != hash {
		t.Errorf("SymptomHash = %q, want %q", exact.SymptomHash, hash)
	}
	if exact.Operator != "sriov-network-operator" {
		t.Errorf("Operator = %q, want %q", exact.Operator, "sriov-network-operator")
	}
	if exact.RootCause != "Missing CRD dependency" {
		t.Errorf("RootCause = %q, want %q", exact.RootCause, "Missing CRD dependency")
	}
	if exact.Classification != "configuration" {
		t.Errorf("Classification = %q, want %q", exact.Classification, "configuration")
	}
	if exact.Resolution != "Install missing CRD from dependency operator" {
		t.Errorf("Resolution = %q, want %q", exact.Resolution, "Install missing CRD from dependency operator")
	}
}

func TestFindSimilarIssuesPartial(t *testing.T) {
	store := openTestStore(t)

	// Save a fingerprint with known symptoms.
	storedSymptoms := []string{
		"state:failed",
		"health:crd established status:fail",
		"health:deployment availability:fail",
		"pattern:missing_guard",
	}
	storedHash := metadata.ComputeSymptomHash(storedSymptoms)

	saved := metadata.Fingerprint{
		SymptomHash:    storedHash,
		Operator:       "sriov-network-operator",
		Symptoms:       storedSymptoms,
		RootCause:      "CRD not installed",
		Classification: "configuration",
		Confidence:     0.8,
	}
	_, err := store.SaveFingerprint(saved)
	if err != nil {
		t.Fatalf("SaveFingerprint: %v", err)
	}

	// Query with overlapping but not identical symptoms.
	// Shares 2 of 4 stored symptoms, plus 1 unique = Jaccard = 2/5 = 0.4
	querySymptoms := []string{
		"state:failed",
		"health:crd established status:fail",
		"pattern:race_condition",
	}
	queryHash := metadata.ComputeSymptomHash(querySymptoms)

	query := metadata.Fingerprint{
		SymptomHash: queryHash,
		Operator:    "sriov-network-operator",
		Symptoms:    querySymptoms,
	}

	results, err := FindSimilarIssues(store, query)
	if err != nil {
		t.Fatalf("FindSimilarIssues: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least 1 fuzzy match result, got 0")
	}

	// Should not be an exact match (different hash).
	found := false
	for _, r := range results {
		if r.SymptomHash == storedHash {
			found = true
			if r.Similarity == 1.0 {
				t.Error("partial match should not have Similarity 1.0")
			}
			if r.Similarity < 0.4 {
				t.Errorf("Similarity = %f, expected >= 0.4 (operator-scoped threshold)", r.Similarity)
			}
			if r.Similarity > 1.0 {
				t.Errorf("Similarity = %f, should not exceed 1.0", r.Similarity)
			}
		}
	}
	if !found {
		t.Error("stored fingerprint not found among fuzzy matches")
	}
}

func TestFindSimilarIssuesEmpty(t *testing.T) {
	store := openTestStore(t)

	// Query against an empty store.
	query := metadata.Fingerprint{
		SymptomHash: metadata.ComputeSymptomHash([]string{"state:failed"}),
		Operator:    "some-operator",
		Symptoms:    []string{"state:failed"},
	}

	results, err := FindSimilarIssues(store, query)
	if err != nil {
		t.Fatalf("FindSimilarIssues on empty store: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results from empty store, got %d", len(results))
	}
}

func TestBuildInsights(t *testing.T) {
	store := openTestStore(t)
	operator := "sriov-network-operator"

	// Pre-populate pattern stats.
	if err := store.RecordPattern("MISSING_GUARD", operator, "test-cluster", 0.8); err != nil {
		t.Fatalf("RecordPattern: %v", err)
	}
	if err := store.RecordPattern("MISSING_GUARD", operator, "test-cluster", 0.9); err != nil {
		t.Fatalf("RecordPattern (second): %v", err)
	}
	if err := store.RecordPattern("RACE_CONDITION", operator, "test-cluster", 0.6); err != nil {
		t.Fatalf("RecordPattern (race): %v", err)
	}

	// Build similar issues with an exact match at position 0.
	similarIssues := []SimilarIssue{
		{
			SymptomHash:    "abc123",
			Operator:       operator,
			Symptoms:       []string{"state:failed", "pattern:missing_guard"},
			RootCause:      "Missing CRD",
			Classification: "configuration",
			Similarity:     1.0,
			HitCount:       5,
		},
		{
			SymptomHash: "def456",
			Operator:    operator,
			Symptoms:    []string{"state:failed"},
			Similarity:  0.6,
			HitCount:    2,
		},
	}

	insights, err := BuildInsights(store, operator, similarIssues)
	if err != nil {
		t.Fatalf("BuildInsights: %v", err)
	}

	// ExactMatch should be set since first issue has Similarity 1.0.
	if insights.ExactMatch == nil {
		t.Fatal("ExactMatch should not be nil when first issue has Similarity 1.0")
	}
	if insights.ExactMatch.SymptomHash != "abc123" {
		t.Errorf("ExactMatch.SymptomHash = %q, want %q", insights.ExactMatch.SymptomHash, "abc123")
	}
	if insights.ExactMatch.RootCause != "Missing CRD" {
		t.Errorf("ExactMatch.RootCause = %q, want %q", insights.ExactMatch.RootCause, "Missing CRD")
	}

	// SimilarIssues should be passed through.
	if len(insights.SimilarIssues) != 2 {
		t.Errorf("SimilarIssues count = %d, want 2", len(insights.SimilarIssues))
	}

	// TopPatterns should contain the patterns we recorded.
	if len(insights.TopPatterns) == 0 {
		t.Fatal("TopPatterns should not be empty after recording patterns")
	}

	// Verify MISSING_GUARD appears with count >= 2.
	foundMG := false
	for _, ps := range insights.TopPatterns {
		if ps.Pattern == "MISSING_GUARD" {
			foundMG = true
			if ps.Count < 2 {
				t.Errorf("MISSING_GUARD count = %d, want >= 2", ps.Count)
			}
		}
	}
	if !foundMG {
		t.Error("TopPatterns should include MISSING_GUARD")
	}

	// Test without exact match: first issue has Similarity < 1.0.
	noExact := []SimilarIssue{
		{
			SymptomHash: "ghi789",
			Operator:    operator,
			Similarity:  0.7,
		},
	}
	insights2, err := BuildInsights(store, operator, noExact)
	if err != nil {
		t.Fatalf("BuildInsights (no exact): %v", err)
	}
	if insights2.ExactMatch != nil {
		t.Error("ExactMatch should be nil when first issue Similarity != 1.0")
	}
}
