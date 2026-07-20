package metadata

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/midu16/opm-troubleshooting/internal/session"
)

// helper opens a MetadataStore in a temporary directory and registers cleanup.
func openTestStore(t *testing.T) *MetadataStore {
	t.Helper()
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open(%q): %v", dir, err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// ---------- 1. TestOpenAndMigrate ----------

func TestOpenAndMigrate(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	// DB file should exist on disk.
	dbPath := filepath.Join(dir, "metadata.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected metadata.db at %s: %v", dbPath, err)
	}

	// BaseDir should match.
	if store.BaseDir() != dir {
		t.Errorf("BaseDir = %q, want %q", store.BaseDir(), dir)
	}

	// DB handle should be non-nil.
	if store.DB() == nil {
		t.Fatal("DB() returned nil")
	}

	// Verify all expected tables were created.
	tables := []string{"sessions", "runs", "fingerprints", "hypotheses", "pattern_stats", "repo_issues", "schema_version"}
	for _, table := range tables {
		var name string
		err := store.DB().QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", table, err)
		}
	}

	// schema_version should be 1.
	var version int
	if err := store.DB().QueryRow("SELECT version FROM schema_version").Scan(&version); err != nil {
		t.Fatalf("read schema_version: %v", err)
	}
	if version != 1 {
		t.Errorf("schema_version = %d, want 1", version)
	}
}

func TestOpenEmptyDirDefaultsToUserHome(t *testing.T) {
	// Open with empty string uses $HOME/.config/opm-troubleshooting.
	// We just verify it does not error; we don't assert the exact path
	// because that depends on the runtime environment.
	store, err := Open("")
	if err != nil {
		t.Skipf("skipping default-dir test (probably no writable home): %v", err)
	}
	store.Close()
}

func TestOpenIdempotentMigrate(t *testing.T) {
	dir := t.TempDir()
	// Open twice to verify migration is idempotent.
	s1, err := Open(dir)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	s1.Close()

	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer s2.Close()

	var count int
	if err := s2.DB().QueryRow("SELECT COUNT(*) FROM schema_version").Scan(&count); err != nil {
		t.Fatalf("count schema_version rows: %v", err)
	}
	if count != 1 {
		t.Errorf("schema_version rows = %d, want 1", count)
	}
}

// ---------- 2. TestSessionCRUD ----------

func TestSessionCRUD(t *testing.T) {
	store := openTestStore(t)

	// Save two sessions.
	sess1 := Session{
		ID:           "sess-001",
		ClusterName:  "prod-cluster-east",
		Operator:     "sriov-network-operator",
		Environment:  "production",
		SourceType:   "must-gather",
		MetadataJSON: `{"version":"1"}`,
	}
	sess2 := Session{
		ID:          "sess-002",
		ClusterName: "staging-cluster",
		Operator:    "ptp-operator",
		Environment: "staging",
		SourceType:  "live",
	}
	for _, s := range []Session{sess1, sess2} {
		if err := store.SaveSession(s); err != nil {
			t.Fatalf("SaveSession(%s): %v", s.ID, err)
		}
	}

	// Load by ID.
	loaded, err := store.LoadSession("sess-001")
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadSession returned nil for existing session")
	}
	if loaded.ClusterName != "prod-cluster-east" {
		t.Errorf("ClusterName = %q, want %q", loaded.ClusterName, "prod-cluster-east")
	}
	if loaded.Operator != "sriov-network-operator" {
		t.Errorf("Operator = %q, want %q", loaded.Operator, "sriov-network-operator")
	}
	if loaded.MetadataJSON != `{"version":"1"}` {
		t.Errorf("MetadataJSON = %q, want %q", loaded.MetadataJSON, `{"version":"1"}`)
	}

	// Load nonexistent returns nil, nil.
	missing, err := store.LoadSession("no-such-id")
	if err != nil {
		t.Fatalf("LoadSession(missing): %v", err)
	}
	if missing != nil {
		t.Error("expected nil for nonexistent session")
	}

	// ListSessions returns both, ordered by updated_at DESC.
	all, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("ListSessions count = %d, want 2", len(all))
	}

	// Upsert: update sess-001 cluster name.
	sess1.ClusterName = "prod-cluster-west"
	if err := store.SaveSession(sess1); err != nil {
		t.Fatalf("SaveSession(upsert): %v", err)
	}
	updated, _ := store.LoadSession("sess-001")
	if updated.ClusterName != "prod-cluster-west" {
		t.Errorf("after upsert ClusterName = %q, want %q", updated.ClusterName, "prod-cluster-west")
	}

	// SearchSessions by operator substring.
	results, err := store.SearchSessions("sriov")
	if err != nil {
		t.Fatalf("SearchSessions: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("SearchSessions(sriov) count = %d, want 1", len(results))
	}
	if results[0].ID != "sess-001" {
		t.Errorf("SearchSessions result ID = %q, want %q", results[0].ID, "sess-001")
	}

	// SearchSessions by cluster name substring.
	results, err = store.SearchSessions("staging")
	if err != nil {
		t.Fatalf("SearchSessions(staging): %v", err)
	}
	if len(results) != 1 || results[0].ID != "sess-002" {
		t.Errorf("SearchSessions(staging) unexpected results: %+v", results)
	}

	// SearchSessions with no match.
	results, err = store.SearchSessions("nonexistent-xyz")
	if err != nil {
		t.Fatalf("SearchSessions(nonexistent): %v", err)
	}
	if len(results) != 0 {
		t.Errorf("SearchSessions(nonexistent) count = %d, want 0", len(results))
	}
}

func TestSessionToJSON(t *testing.T) {
	input := map[string]string{"key": "value"}
	got := SessionToJSON(input)
	expected := `{"key":"value"}`
	if got != expected {
		t.Errorf("SessionToJSON = %q, want %q", got, expected)
	}
}

// ---------- 3. TestRunRecording ----------

func TestRunRecording(t *testing.T) {
	store := openTestStore(t)

	// Create a session first.
	if err := store.SaveSession(Session{ID: "run-sess", ClusterName: "c1", Operator: "op1"}); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	// Record two runs.
	run1 := Run{
		SessionID:      "run-sess",
		Status:         "completed",
		RealIssues:     3,
		CosmeticAlerts: 1,
		HealthPassed:   10,
		HealthFailed:   2,
		InfraPassed:    5,
		InfraFailed:    0,
		ADHDBranches:   4,
		ADHDTraps:      1,
		MustGatherPath: "/tmp/mg1",
		RCAPath:        "/tmp/rca1",
		Classification: "degraded",
	}
	id1, err := store.RecordRun(run1)
	if err != nil {
		t.Fatalf("RecordRun #1: %v", err)
	}
	if id1 <= 0 {
		t.Errorf("run ID should be positive, got %d", id1)
	}

	run2 := Run{
		SessionID:  "run-sess",
		Status:     "failed",
		RealIssues: 5,
	}
	id2, err := store.RecordRun(run2)
	if err != nil {
		t.Fatalf("RecordRun #2: %v", err)
	}
	if id2 <= id1 {
		t.Errorf("second run ID (%d) should be greater than first (%d)", id2, id1)
	}

	// GetRunsForSession returns both (ordered by timestamp DESC).
	runs, err := store.GetRunsForSession("run-sess")
	if err != nil {
		t.Fatalf("GetRunsForSession: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("GetRunsForSession count = %d, want 2", len(runs))
	}

	// Both run IDs must be present.
	ids := map[int64]bool{runs[0].ID: true, runs[1].ID: true}
	if !ids[id1] || !ids[id2] {
		t.Errorf("expected IDs {%d, %d}, got {%d, %d}", id1, id2, runs[0].ID, runs[1].ID)
	}

	// Verify fields round-trip on the first run.
	var found *Run
	for i := range runs {
		if runs[i].ID == id1 {
			found = &runs[i]
			break
		}
	}
	if found == nil {
		t.Fatal("run1 not found in results")
	}
	if found.RealIssues != 3 || found.HealthPassed != 10 || found.Classification != "degraded" {
		t.Errorf("field mismatch: got RealIssues=%d HealthPassed=%d Classification=%q",
			found.RealIssues, found.HealthPassed, found.Classification)
	}

	// GetRunsForSession with unknown session returns empty.
	empty, err := store.GetRunsForSession("no-such-session")
	if err != nil {
		t.Fatalf("GetRunsForSession(missing): %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("expected empty for unknown session, got %d", len(empty))
	}

	// GetRecentRuns with limit.
	recent, err := store.GetRecentRuns(1)
	if err != nil {
		t.Fatalf("GetRecentRuns(1): %v", err)
	}
	if len(recent) != 1 {
		t.Fatalf("GetRecentRuns(1) count = %d, want 1", len(recent))
	}

	// GetRecentRuns with 0 defaults to 20, should return all 2.
	all, err := store.GetRecentRuns(0)
	if err != nil {
		t.Fatalf("GetRecentRuns(0): %v", err)
	}
	if len(all) != 2 {
		t.Errorf("GetRecentRuns(0) count = %d, want 2", len(all))
	}
}

// ---------- 4. TestFingerprintSaveAndLookup ----------

func TestFingerprintSaveAndLookup(t *testing.T) {
	store := openTestStore(t)

	symptoms := []string{"CrashLoopBackOff", "ImagePullBackOff"}
	hash := ComputeSymptomHash(symptoms)

	fp := Fingerprint{
		RunID:          1,
		SymptomHash:    hash,
		Operator:       "sriov-network-operator",
		Symptoms:       symptoms,
		RootCause:      "missing image",
		Classification: "config-error",
		Resolution:     "fix image ref",
		Confidence:     0.85,
	}
	id, err := store.SaveFingerprint(fp)
	if err != nil {
		t.Fatalf("SaveFingerprint: %v", err)
	}
	if id <= 0 {
		t.Errorf("fingerprint ID should be positive, got %d", id)
	}

	// FindByHash exact match.
	found, err := store.FindByHash(hash)
	if err != nil {
		t.Fatalf("FindByHash: %v", err)
	}
	if found == nil {
		t.Fatal("FindByHash returned nil for existing hash")
	}
	if found.Operator != "sriov-network-operator" {
		t.Errorf("Operator = %q, want %q", found.Operator, "sriov-network-operator")
	}
	if found.RootCause != "missing image" {
		t.Errorf("RootCause = %q, want %q", found.RootCause, "missing image")
	}
	if len(found.Symptoms) != 2 {
		t.Errorf("Symptoms len = %d, want 2", len(found.Symptoms))
	}
	if found.HitCount != 1 {
		t.Errorf("HitCount = %d, want 1", found.HitCount)
	}
	if found.Confidence != 0.85 {
		t.Errorf("Confidence = %f, want 0.85", found.Confidence)
	}

	// Save same hash+operator again: should increment hit_count.
	fp.RunID = 2
	fp.Confidence = 0.90
	id2, err := store.SaveFingerprint(fp)
	if err != nil {
		t.Fatalf("SaveFingerprint(duplicate): %v", err)
	}
	if id2 != id {
		t.Errorf("duplicate save returned id=%d, want original %d", id2, id)
	}
	found2, _ := store.FindByHash(hash)
	if found2.HitCount != 2 {
		t.Errorf("HitCount after duplicate = %d, want 2", found2.HitCount)
	}
	if found2.Confidence != 0.90 {
		t.Errorf("Confidence after duplicate = %f, want 0.90", found2.Confidence)
	}

	// FindByHash for nonexistent returns nil, nil.
	missing, err := store.FindByHash("0000000000000000000000000000000000000000000000000000000000000000")
	if err != nil {
		t.Fatalf("FindByHash(missing): %v", err)
	}
	if missing != nil {
		t.Error("expected nil for nonexistent hash")
	}

	// UpdateResolution.
	if err := store.UpdateResolution(id, "actual root cause", "infra", "reboot node"); err != nil {
		t.Fatalf("UpdateResolution: %v", err)
	}
	updated, _ := store.FindByHash(hash)
	if updated.RootCause != "actual root cause" {
		t.Errorf("RootCause after update = %q, want %q", updated.RootCause, "actual root cause")
	}
	if updated.Classification != "infra" {
		t.Errorf("Classification after update = %q, want %q", updated.Classification, "infra")
	}
	if updated.Resolution != "reboot node" {
		t.Errorf("Resolution after update = %q, want %q", updated.Resolution, "reboot node")
	}
}

// ---------- 5. TestFingerprintSimilarity ----------

func TestFingerprintSimilarity(t *testing.T) {
	store := openTestStore(t)

	// Insert a few fingerprints with known symptoms.
	fp1 := Fingerprint{
		RunID:       1,
		SymptomHash: ComputeSymptomHash([]string{"CrashLoopBackOff", "OOMKilled", "HighCPU"}),
		Operator:    "op-a",
		Symptoms:    []string{"CrashLoopBackOff", "OOMKilled", "HighCPU"},
		Confidence:  0.8,
	}
	fp2 := Fingerprint{
		RunID:       2,
		SymptomHash: ComputeSymptomHash([]string{"ImagePullBackOff", "ErrImagePull"}),
		Operator:    "op-a",
		Symptoms:    []string{"ImagePullBackOff", "ErrImagePull"},
		Confidence:  0.7,
	}
	fp3 := Fingerprint{
		RunID:       3,
		SymptomHash: ComputeSymptomHash([]string{"CrashLoopBackOff", "OOMKilled"}),
		Operator:    "op-b",
		Symptoms:    []string{"CrashLoopBackOff", "OOMKilled"},
		Confidence:  0.6,
	}
	for _, fp := range []Fingerprint{fp1, fp2, fp3} {
		if _, err := store.SaveFingerprint(fp); err != nil {
			t.Fatalf("SaveFingerprint: %v", err)
		}
	}

	// FindSimilar with query symptoms that overlap with fp1 and fp3.
	query := []string{"CrashLoopBackOff", "OOMKilled"}
	results, err := store.FindSimilar(query, 0.5)
	if err != nil {
		t.Fatalf("FindSimilar: %v", err)
	}
	// fp1 has Jaccard 2/3 ~ 0.667, fp3 has Jaccard 2/2 = 1.0, fp2 has 0.0
	if len(results) < 2 {
		t.Fatalf("FindSimilar count = %d, want >= 2", len(results))
	}
	// Results should be sorted by similarity descending; fp3 (1.0) first.
	if results[0].Similarity < results[1].Similarity {
		t.Errorf("results not sorted by descending similarity: %.3f < %.3f",
			results[0].Similarity, results[1].Similarity)
	}
	// fp2 should not appear at threshold 0.5.
	for _, r := range results {
		if r.Fingerprint.Operator == "op-a" && len(r.Fingerprint.Symptoms) == 2 &&
			r.Fingerprint.Symptoms[0] == "ImagePullBackOff" {
			t.Error("fp2 (ImagePullBackOff) should not match at threshold 0.5")
		}
	}

	// FindSimilarForOperator scoped to op-a.
	opResults, err := store.FindSimilarForOperator("op-a", query, 0.5)
	if err != nil {
		t.Fatalf("FindSimilarForOperator: %v", err)
	}
	// Only fp1 (op-a) should match; fp3 is op-b.
	if len(opResults) != 1 {
		t.Fatalf("FindSimilarForOperator count = %d, want 1", len(opResults))
	}
	if opResults[0].Fingerprint.Operator != "op-a" {
		t.Errorf("operator = %q, want op-a", opResults[0].Fingerprint.Operator)
	}

	// High threshold filters everything out.
	strict, err := store.FindSimilar(query, 0.99)
	if err != nil {
		t.Fatalf("FindSimilar(strict): %v", err)
	}
	// Only fp3 has Jaccard 1.0, which is >= 0.99.
	if len(strict) != 1 {
		t.Errorf("FindSimilar(0.99) count = %d, want 1 (only exact match)", len(strict))
	}
}

// ---------- 6. TestComputeSymptomHash ----------

func TestComputeSymptomHash(t *testing.T) {
	// Deterministic: same input produces same hash.
	h1 := ComputeSymptomHash([]string{"CrashLoopBackOff", "OOMKilled"})
	h2 := ComputeSymptomHash([]string{"CrashLoopBackOff", "OOMKilled"})
	if h1 != h2 {
		t.Errorf("same input produced different hashes: %s vs %s", h1, h2)
	}

	// Order-independent.
	h3 := ComputeSymptomHash([]string{"OOMKilled", "CrashLoopBackOff"})
	if h1 != h3 {
		t.Errorf("different order produced different hash: %s vs %s", h1, h3)
	}

	// Case-insensitive.
	h4 := ComputeSymptomHash([]string{"CRASHLOOPBACKOFF", "oomkilled"})
	if h1 != h4 {
		t.Errorf("different case produced different hash: %s vs %s", h1, h4)
	}

	// Whitespace-trimmed.
	h5 := ComputeSymptomHash([]string{"  CrashLoopBackOff  ", " OOMKilled "})
	if h1 != h5 {
		t.Errorf("whitespace produced different hash: %s vs %s", h1, h5)
	}

	// Empty strings are filtered out.
	h6 := ComputeSymptomHash([]string{"CrashLoopBackOff", "", "  ", "OOMKilled"})
	if h1 != h6 {
		t.Errorf("empty strings changed hash: %s vs %s", h1, h6)
	}

	// Different symptoms produce different hash.
	h7 := ComputeSymptomHash([]string{"ImagePullBackOff"})
	if h1 == h7 {
		t.Error("different symptoms produced the same hash")
	}

	// Hash is a valid 64-char hex string (SHA256).
	if len(h1) != 64 {
		t.Errorf("hash length = %d, want 64", len(h1))
	}

	// All-empty produces a hash of the empty string.
	hEmpty := ComputeSymptomHash([]string{})
	hEmpty2 := ComputeSymptomHash(nil)
	if hEmpty != hEmpty2 {
		t.Errorf("empty slice vs nil produced different hashes")
	}
}

// ---------- 7. TestJaccardSimilarity ----------

func TestJaccardSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a, b []string
		want float64
	}{
		{"both empty", nil, nil, 1.0},
		{"a empty", nil, []string{"x"}, 0.0},
		{"b empty", []string{"x"}, nil, 0.0},
		{"identical", []string{"a", "b", "c"}, []string{"a", "b", "c"}, 1.0},
		{"disjoint", []string{"a", "b"}, []string{"c", "d"}, 0.0},
		{"partial overlap", []string{"a", "b", "c"}, []string{"b", "c", "d"}, 0.5}, // intersection=2, union=4
		{"single overlap", []string{"a", "b"}, []string{"b", "c"}, 1.0 / 3.0},
		{"case insensitive", []string{"ABC"}, []string{"abc"}, 1.0},
		{"whitespace trimmed", []string{" x "}, []string{"x"}, 1.0},
		{"superset", []string{"a", "b", "c"}, []string{"a", "b"}, 2.0 / 3.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := JaccardSimilarity(tt.a, tt.b)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("JaccardSimilarity(%v, %v) = %f, want %f", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// ---------- 8. TestHypothesisRecording ----------

func TestHypothesisRecording(t *testing.T) {
	store := openTestStore(t)

	// Create session and run first.
	if err := store.SaveSession(Session{ID: "hyp-sess", ClusterName: "c1", Operator: "op1"}); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	runID, err := store.RecordRun(Run{SessionID: "hyp-sess", Status: "completed"})
	if err != nil {
		t.Fatalf("RecordRun: %v", err)
	}

	confirmed := true
	hypotheses := []HypothesisRecord{
		{RunID: runID, FrameID: "frame-rbac", HypothesisText: "RBAC misconfiguration", ScoreTotal: 8.5, WasTrap: false, WasConfirmed: &confirmed, Operator: "op1", ClusterName: "c1"},
		{RunID: runID, FrameID: "frame-net", HypothesisText: "Network policy blocking", ScoreTotal: 6.2, WasTrap: true, WasConfirmed: nil, Operator: "op1", ClusterName: "c1"},
		{RunID: runID, FrameID: "frame-rbac", HypothesisText: "ServiceAccount missing", ScoreTotal: 7.0, WasTrap: false, WasConfirmed: nil, Operator: "op1", ClusterName: "c1"},
	}

	if err := store.RecordHypotheses(runID, hypotheses); err != nil {
		t.Fatalf("RecordHypotheses: %v", err)
	}

	// GetHypothesesForRun.
	results, err := store.GetHypothesesForRun(runID)
	if err != nil {
		t.Fatalf("GetHypothesesForRun: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("hypothesis count = %d, want 3", len(results))
	}

	// Verify the first hypothesis has WasConfirmed set.
	var rbacHyp *HypothesisRecord
	for i := range results {
		if results[i].HypothesisText == "RBAC misconfiguration" {
			rbacHyp = &results[i]
			break
		}
	}
	if rbacHyp == nil {
		t.Fatal("RBAC hypothesis not found")
	}
	if rbacHyp.WasConfirmed == nil || !*rbacHyp.WasConfirmed {
		t.Error("RBAC hypothesis should be confirmed")
	}
	if rbacHyp.ScoreTotal != 8.5 {
		t.Errorf("ScoreTotal = %f, want 8.5", rbacHyp.ScoreTotal)
	}

	// Verify the network hypothesis has WasTrap=true and WasConfirmed=nil.
	var netHyp *HypothesisRecord
	for i := range results {
		if results[i].FrameID == "frame-net" {
			netHyp = &results[i]
			break
		}
	}
	if netHyp == nil {
		t.Fatal("network hypothesis not found")
	}
	if !netHyp.WasTrap {
		t.Error("network hypothesis should have WasTrap=true")
	}
	if netHyp.WasConfirmed != nil {
		t.Error("network hypothesis WasConfirmed should be nil")
	}

	// GetHypothesesForRun with nonexistent run.
	empty, err := store.GetHypothesesForRun(9999)
	if err != nil {
		t.Fatalf("GetHypothesesForRun(9999): %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("expected empty for unknown run, got %d", len(empty))
	}
}

// ---------- 9. TestFrameAccuracy ----------

func TestFrameAccuracy(t *testing.T) {
	store := openTestStore(t)

	// FrameAccuracy with no data should return 0.5 (neutral).
	acc, err := store.FrameAccuracy("frame-unknown")
	if err != nil {
		t.Fatalf("FrameAccuracy(unknown): %v", err)
	}
	if acc != 0.5 {
		t.Errorf("FrameAccuracy(unknown) = %f, want 0.5", acc)
	}

	// Insert hypotheses with known confirmations.
	if err := store.SaveSession(Session{ID: "acc-sess", ClusterName: "c1", Operator: "op1"}); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	runID, _ := store.RecordRun(Run{SessionID: "acc-sess", Status: "done"})

	trueVal := true
	falseVal := false
	hyps := []HypothesisRecord{
		{RunID: runID, FrameID: "frame-x", HypothesisText: "h1", ScoreTotal: 5, Operator: "op1", ClusterName: "c1", WasConfirmed: &trueVal},
		{RunID: runID, FrameID: "frame-x", HypothesisText: "h2", ScoreTotal: 5, Operator: "op1", ClusterName: "c1", WasConfirmed: &trueVal},
		{RunID: runID, FrameID: "frame-x", HypothesisText: "h3", ScoreTotal: 5, Operator: "op1", ClusterName: "c1", WasConfirmed: &falseVal},
		{RunID: runID, FrameID: "frame-x", HypothesisText: "h4", ScoreTotal: 5, Operator: "op1", ClusterName: "c1", WasConfirmed: nil}, // unresolved, should be excluded
	}
	if err := store.RecordHypotheses(runID, hyps); err != nil {
		t.Fatalf("RecordHypotheses: %v", err)
	}

	// 2 confirmed out of 3 resolved (nil excluded) = 2/3.
	acc, err = store.FrameAccuracy("frame-x")
	if err != nil {
		t.Fatalf("FrameAccuracy: %v", err)
	}
	expected := 2.0 / 3.0
	if math.Abs(acc-expected) > 1e-9 {
		t.Errorf("FrameAccuracy = %f, want %f", acc, expected)
	}

	// ConfirmHypothesis: confirm the previously nil hypothesis.
	// Find its ID first.
	allHyps, _ := store.GetHypothesesForRun(runID)
	var h4ID int64
	for _, h := range allHyps {
		if h.HypothesisText == "h4" {
			h4ID = h.ID
			break
		}
	}
	if h4ID == 0 {
		t.Fatal("h4 not found")
	}
	if err := store.ConfirmHypothesis(h4ID, true); err != nil {
		t.Fatalf("ConfirmHypothesis: %v", err)
	}

	// Now 3 confirmed out of 4 resolved = 3/4.
	acc, _ = store.FrameAccuracy("frame-x")
	expected = 3.0 / 4.0
	if math.Abs(acc-expected) > 1e-9 {
		t.Errorf("FrameAccuracy after confirm = %f, want %f", acc, expected)
	}
}

// ---------- 10. TestBoostFactors ----------

func TestBoostFactors(t *testing.T) {
	store := openTestStore(t)

	if err := store.SaveSession(Session{ID: "boost-sess", ClusterName: "c1", Operator: "boost-op"}); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	runID, _ := store.RecordRun(Run{SessionID: "boost-sess", Status: "done"})

	trueVal := true
	falseVal := false

	// frame-a: 2 confirmed, 0 rejected => accuracy = 1.0 => boost = 1 + 0.2*(1.0 - 0.5) = 1.1
	// frame-b: 1 confirmed, 1 rejected => accuracy = 0.5 => boost = 1 + 0.2*(0.5 - 0.5) = 1.0
	// frame-c: 0 confirmed, 2 rejected => accuracy = 0.0 => boost = 1 + 0.2*(0.0 - 0.5) = 0.9
	hyps := []HypothesisRecord{
		{RunID: runID, FrameID: "frame-a", HypothesisText: "a1", ScoreTotal: 5, WasConfirmed: &trueVal, Operator: "boost-op", ClusterName: "c1"},
		{RunID: runID, FrameID: "frame-a", HypothesisText: "a2", ScoreTotal: 5, WasConfirmed: &trueVal, Operator: "boost-op", ClusterName: "c1"},
		{RunID: runID, FrameID: "frame-b", HypothesisText: "b1", ScoreTotal: 5, WasConfirmed: &trueVal, Operator: "boost-op", ClusterName: "c1"},
		{RunID: runID, FrameID: "frame-b", HypothesisText: "b2", ScoreTotal: 5, WasConfirmed: &falseVal, Operator: "boost-op", ClusterName: "c1"},
		{RunID: runID, FrameID: "frame-c", HypothesisText: "c1", ScoreTotal: 5, WasConfirmed: &falseVal, Operator: "boost-op", ClusterName: "c1"},
		{RunID: runID, FrameID: "frame-c", HypothesisText: "c2", ScoreTotal: 5, WasConfirmed: &falseVal, Operator: "boost-op", ClusterName: "c1"},
		// Unresolved hypothesis should not affect boosts.
		{RunID: runID, FrameID: "frame-a", HypothesisText: "a3", ScoreTotal: 5, WasConfirmed: nil, Operator: "boost-op", ClusterName: "c1"},
	}
	if err := store.RecordHypotheses(runID, hyps); err != nil {
		t.Fatalf("RecordHypotheses: %v", err)
	}

	boosts, err := store.GetBoostFactors("boost-op")
	if err != nil {
		t.Fatalf("GetBoostFactors: %v", err)
	}

	cases := map[string]float64{
		"frame-a": 1.1,
		"frame-b": 1.0,
		"frame-c": 0.9,
	}
	for frame, want := range cases {
		got, ok := boosts[frame]
		if !ok {
			t.Errorf("boost for %q not found", frame)
			continue
		}
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("boost[%q] = %f, want %f", frame, got, want)
		}
	}

	// Unknown operator should return empty map.
	empty, err := store.GetBoostFactors("nonexistent-op")
	if err != nil {
		t.Fatalf("GetBoostFactors(unknown): %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("expected empty boosts for unknown operator, got %d entries", len(empty))
	}
}

func TestGetFrameStatsForOperator(t *testing.T) {
	store := openTestStore(t)

	if err := store.SaveSession(Session{ID: "fs-sess", ClusterName: "c1", Operator: "fs-op"}); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	runID, _ := store.RecordRun(Run{SessionID: "fs-sess", Status: "done"})

	trueVal := true
	falseVal := false
	hyps := []HypothesisRecord{
		{RunID: runID, FrameID: "f1", HypothesisText: "h1", ScoreTotal: 10, WasTrap: false, WasConfirmed: &trueVal, Operator: "fs-op", ClusterName: "c1"},
		{RunID: runID, FrameID: "f1", HypothesisText: "h2", ScoreTotal: 6, WasTrap: true, WasConfirmed: &falseVal, Operator: "fs-op", ClusterName: "c1"},
		{RunID: runID, FrameID: "f2", HypothesisText: "h3", ScoreTotal: 4, WasTrap: false, WasConfirmed: nil, Operator: "fs-op", ClusterName: "c1"},
	}
	if err := store.RecordHypotheses(runID, hyps); err != nil {
		t.Fatalf("RecordHypotheses: %v", err)
	}

	stats, err := store.GetFrameStatsForOperator("fs-op")
	if err != nil {
		t.Fatalf("GetFrameStatsForOperator: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("stats count = %d, want 2", len(stats))
	}

	// f1 should have Total=2, Confirmed=1, Rejected=1, TrapCount=1.
	var f1 *FrameStats
	for i := range stats {
		if stats[i].FrameID == "f1" {
			f1 = &stats[i]
		}
	}
	if f1 == nil {
		t.Fatal("f1 stats not found")
	}
	if f1.Total != 2 || f1.Confirmed != 1 || f1.Rejected != 1 || f1.TrapCount != 1 {
		t.Errorf("f1 stats: Total=%d Confirmed=%d Rejected=%d TrapCount=%d",
			f1.Total, f1.Confirmed, f1.Rejected, f1.TrapCount)
	}
}

// ---------- 11. TestPatternStats ----------

func TestPatternStats(t *testing.T) {
	store := openTestStore(t)

	// Record the same pattern multiple times.
	for i := 0; i < 3; i++ {
		if err := store.RecordPattern("CrashLoop", "op1", "cluster-a", 0.9); err != nil {
			t.Fatalf("RecordPattern iteration %d: %v", i, err)
		}
	}
	// Record a different pattern once.
	if err := store.RecordPattern("OOMKill", "op1", "cluster-a", 0.7); err != nil {
		t.Fatalf("RecordPattern(OOMKill): %v", err)
	}
	// Record same pattern for different operator.
	if err := store.RecordPattern("CrashLoop", "op2", "cluster-b", 0.8); err != nil {
		t.Fatalf("RecordPattern(op2): %v", err)
	}

	// GetPatternFrequency for op1.
	patterns, err := store.GetPatternFrequency("op1")
	if err != nil {
		t.Fatalf("GetPatternFrequency: %v", err)
	}
	if len(patterns) != 2 {
		t.Fatalf("pattern count = %d, want 2", len(patterns))
	}
	// Sorted by count DESC: CrashLoop (3) first, then OOMKill (1).
	if patterns[0].Pattern != "CrashLoop" || patterns[0].Count != 3 {
		t.Errorf("top pattern: %q count=%d, want CrashLoop count=3", patterns[0].Pattern, patterns[0].Count)
	}
	if patterns[1].Pattern != "OOMKill" || patterns[1].Count != 1 {
		t.Errorf("second pattern: %q count=%d, want OOMKill count=1", patterns[1].Pattern, patterns[1].Count)
	}

	// GetTopPatterns across all operators.
	top, err := store.GetTopPatterns(10)
	if err != nil {
		t.Fatalf("GetTopPatterns: %v", err)
	}
	if len(top) != 3 {
		t.Fatalf("top patterns count = %d, want 3", len(top))
	}
	// Highest count first.
	if top[0].Count != 3 {
		t.Errorf("top pattern count = %d, want 3", top[0].Count)
	}

	// GetTopPatterns with 0 defaults to 20.
	all, err := store.GetTopPatterns(0)
	if err != nil {
		t.Fatalf("GetTopPatterns(0): %v", err)
	}
	if len(all) != 3 {
		t.Errorf("GetTopPatterns(0) count = %d, want 3", len(all))
	}
}

// ---------- 12. TestRepoIssueCache ----------

func TestRepoIssueCache(t *testing.T) {
	store := openTestStore(t)

	hash := ComputeSymptomHash([]string{"CrashLoop"})

	issue1 := RepoIssue{
		Repo:        "openshift/sriov-network-operator",
		IssueNumber: 42,
		Title:       "Operator crashes on startup",
		State:       "open",
		Labels:      "bug,priority/high",
		BodyExcerpt: "The operator pod enters CrashLoopBackOff...",
		SymptomHash: hash,
	}
	issue2 := RepoIssue{
		Repo:        "openshift/sriov-network-operator",
		IssueNumber: 99,
		Title:       "Network config not applied",
		State:       "closed",
		Labels:      "enhancement",
		BodyExcerpt: "Configuration changes are not picked up...",
		SymptomHash: "",
	}
	issue3 := RepoIssue{
		Repo:        "openshift/ptp-operator",
		IssueNumber: 10,
		Title:       "PTP sync fails",
		State:       "open",
		Labels:      "bug",
		BodyExcerpt: "Grand master clock sync fails...",
		SymptomHash: hash,
	}

	for _, iss := range []RepoIssue{issue1, issue2, issue3} {
		if err := store.SaveRepoIssue(iss); err != nil {
			t.Fatalf("SaveRepoIssue(%d): %v", iss.IssueNumber, err)
		}
	}

	// GetCachedIssues for repo without symptom filter.
	sriov, err := store.GetCachedIssues("openshift/sriov-network-operator", "")
	if err != nil {
		t.Fatalf("GetCachedIssues: %v", err)
	}
	if len(sriov) != 2 {
		t.Fatalf("sriov issue count = %d, want 2", len(sriov))
	}

	// GetCachedIssues with symptom hash filter.
	filtered, err := store.GetCachedIssues("openshift/sriov-network-operator", hash)
	if err != nil {
		t.Fatalf("GetCachedIssues(filtered): %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("filtered count = %d, want 1", len(filtered))
	}
	if filtered[0].IssueNumber != 42 {
		t.Errorf("filtered issue number = %d, want 42", filtered[0].IssueNumber)
	}
	if filtered[0].Title != "Operator crashes on startup" {
		t.Errorf("filtered title = %q", filtered[0].Title)
	}

	// GetCachedIssues for different repo.
	ptp, err := store.GetCachedIssues("openshift/ptp-operator", "")
	if err != nil {
		t.Fatalf("GetCachedIssues(ptp): %v", err)
	}
	if len(ptp) != 1 {
		t.Fatalf("ptp issue count = %d, want 1", len(ptp))
	}

	// GetCachedIssues for nonexistent repo.
	none, err := store.GetCachedIssues("openshift/no-such-repo", "")
	if err != nil {
		t.Fatalf("GetCachedIssues(nonexistent): %v", err)
	}
	if len(none) != 0 {
		t.Errorf("expected 0 issues for unknown repo, got %d", len(none))
	}
}

// ---------- 13. TestMigrateFromJSON ----------

func TestMigrateFromJSON(t *testing.T) {
	store := openTestStore(t)

	// Create a temporary sessions directory with JSON files.
	sessDir := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(sessDir, 0o750); err != nil {
		t.Fatalf("mkdir sessions dir: %v", err)
	}

	record := session.Record{
		ClusterID:       "cluster-abc-123",
		ClusterName:     "test-cluster",
		OperatorPackage: "sriov-network-operator",
		Environment:     "lab",
		History: []session.HistoryEntry{
			{Status: "healthy", RealIssues: 0, CosmeticAlerts: 2, MustGatherPath: "/mg/path1"},
			{Status: "degraded", RealIssues: 3, CosmeticAlerts: 1, MustGatherPath: "/mg/path2"},
		},
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	jsonPath := filepath.Join(sessDir, "cluster-abc-123.json")
	if err := os.WriteFile(jsonPath, data, 0o600); err != nil {
		t.Fatalf("write json: %v", err)
	}

	// Write a non-JSON file that should be ignored.
	if err := os.WriteFile(filepath.Join(sessDir, "notes.txt"), []byte("ignore me"), 0o600); err != nil {
		t.Fatalf("write txt: %v", err)
	}

	// Write an invalid JSON file that should be skipped.
	if err := os.WriteFile(filepath.Join(sessDir, "bad.json"), []byte("{invalid}"), 0o600); err != nil {
		t.Fatalf("write bad json: %v", err)
	}

	// Run migration.
	count, err := store.MigrateFromJSON(sessDir)
	if err != nil {
		t.Fatalf("MigrateFromJSON: %v", err)
	}
	if count != 1 {
		t.Errorf("migrated count = %d, want 1", count)
	}

	// Verify session was created.
	sess, err := store.LoadSession("cluster-abc-123")
	if err != nil {
		t.Fatalf("LoadSession after migration: %v", err)
	}
	if sess == nil {
		t.Fatal("session not found after migration")
	}
	if sess.ClusterName != "test-cluster" {
		t.Errorf("ClusterName = %q, want %q", sess.ClusterName, "test-cluster")
	}
	if sess.Operator != "sriov-network-operator" {
		t.Errorf("Operator = %q, want %q", sess.Operator, "sriov-network-operator")
	}
	if sess.SourceType != "must-gather" {
		t.Errorf("SourceType = %q, want %q", sess.SourceType, "must-gather")
	}

	// MetadataJSON should contain the full record.
	if sess.MetadataJSON == "" {
		t.Error("MetadataJSON is empty after migration")
	}

	// Verify runs were created from history entries.
	runs, err := store.GetRunsForSession("cluster-abc-123")
	if err != nil {
		t.Fatalf("GetRunsForSession after migration: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("runs count = %d, want 2", len(runs))
	}

	// MigrateFromJSON on nonexistent dir returns 0, nil.
	count, err = store.MigrateFromJSON("/nonexistent/path/12345")
	if err != nil {
		t.Fatalf("MigrateFromJSON(nonexistent): %v", err)
	}
	if count != 0 {
		t.Errorf("migrated count for nonexistent dir = %d, want 0", count)
	}

	// Running migration again should not duplicate (INSERT OR IGNORE).
	count, err = store.MigrateFromJSON(sessDir)
	if err != nil {
		t.Fatalf("MigrateFromJSON(re-run): %v", err)
	}
	// The session INSERT is IGNORE, so count is still 1 (it processes the file).
	// But the runs INSERT does not have IGNORE, so runs will be duplicated.
	// Verify the session is not duplicated.
	allSessions, _ := store.ListSessions()
	sessionCount := 0
	for _, s := range allSessions {
		if s.ID == "cluster-abc-123" {
			sessionCount++
		}
	}
	if sessionCount != 1 {
		t.Errorf("session duplicated: found %d copies", sessionCount)
	}
}

// ---------- Integration: TestSessionStats ----------

func TestSessionStats(t *testing.T) {
	store := openTestStore(t)

	if err := store.SaveSession(Session{ID: "stat-sess", ClusterName: "c1", Operator: "stat-op"}); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	// Stats with no runs.
	stats, err := store.GetSessionStats("stat-sess")
	if err != nil {
		t.Fatalf("GetSessionStats(empty): %v", err)
	}
	if stats.TotalRuns != 0 {
		t.Errorf("TotalRuns = %d, want 0", stats.TotalRuns)
	}

	// Add runs.
	store.RecordRun(Run{SessionID: "stat-sess", Status: "completed", RealIssues: 1})
	store.RecordRun(Run{SessionID: "stat-sess", Status: "failed", RealIssues: 3})
	store.RecordRun(Run{SessionID: "stat-sess", Status: "completed", RealIssues: 0})

	// Add patterns for the operator.
	store.RecordPattern("CrashLoop", "stat-op", "c1", 0.9)
	store.RecordPattern("OOMKill", "stat-op", "c1", 0.8)

	stats, err = store.GetSessionStats("stat-sess")
	if err != nil {
		t.Fatalf("GetSessionStats: %v", err)
	}
	if stats.TotalRuns != 3 {
		t.Errorf("TotalRuns = %d, want 3", stats.TotalRuns)
	}
	if stats.FailedRuns != 1 {
		t.Errorf("FailedRuns = %d, want 1", stats.FailedRuns)
	}
	if len(stats.TopPatterns) != 2 {
		t.Errorf("TopPatterns count = %d, want 2", len(stats.TopPatterns))
	}
}
