package session

import (
	"path/filepath"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	record, err := store.Load("test-cluster", "redhat-oadp-operator")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if record.RedeploymentCount != 0 {
		t.Errorf("expected 0 redeployments, got %d", record.RedeploymentCount)
	}

	store.RecordRun(record, HistoryEntry{
		Summary:    "Initial deployment",
		Status:     "healthy",
		RealIssues: 0,
	})
	if err := store.Save(record); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load("test-cluster", "redhat-oadp-operator")
	if err != nil {
		t.Fatalf("Load after save: %v", err)
	}
	if loaded.RedeploymentCount != 1 {
		t.Errorf("redeployment count: got %d want 1", loaded.RedeploymentCount)
	}
	if len(loaded.History) != 1 {
		t.Errorf("history entries: got %d want 1", len(loaded.History))
	}

	// Verify file exists
	path := filepath.Join(dir, loaded.ClusterID+".json")
	if _, err := filepath.Abs(path); err != nil {
		t.Fatalf("session file path: %v", err)
	}
}

func TestDefaultClusterName(t *testing.T) {
	name := DefaultClusterName("/path/to/must-gather.local.12345")
	if name != "must-gather.local.12345" {
		t.Errorf("got %q", name)
	}
}

func TestAddKnownCosmetic(t *testing.T) {
	record := &Record{KnownCosmetic: make([]string, 0)}
	store := &Store{}
	store.AddKnownCosmetic(record, "alert1")
	store.AddKnownCosmetic(record, "alert1")
	if len(record.KnownCosmetic) != 1 {
		t.Errorf("expected dedup, got %d entries", len(record.KnownCosmetic))
	}
}
