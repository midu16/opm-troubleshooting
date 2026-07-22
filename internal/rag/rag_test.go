package rag

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.DataDir != "rag-data" {
		t.Errorf("expected DataDir=rag-data, got %s", cfg.DataDir)
	}
	if cfg.Embedding.Model != "all-minilm" {
		t.Errorf("expected Embedding.Model=all-minilm, got %s", cfg.Embedding.Model)
	}
	if cfg.OpenShift.Version != "4.22" {
		t.Errorf("expected OpenShift.Version=4.22, got %s", cfg.OpenShift.Version)
	}
	if len(cfg.OpenShift.Repos) != 27 {
		t.Errorf("expected 27 repos, got %d", len(cfg.OpenShift.Repos))
	}
	if cfg.Retrieval.DefaultTopK != 8 {
		t.Errorf("expected DefaultTopK=8, got %d", cfg.Retrieval.DefaultTopK)
	}
}

func TestConfigPaths(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ReposDir() != "rag-data/repos" {
		t.Errorf("expected ReposDir=rag-data/repos, got %s", cfg.ReposDir())
	}
	if cfg.DocsDir() != "rag-data/docs" {
		t.Errorf("expected DocsDir=rag-data/docs, got %s", cfg.DocsDir())
	}
	if cfg.ChromemDir() != "rag-data/chromem" {
		t.Errorf("expected ChromemDir=rag-data/chromem, got %s", cfg.ChromemDir())
	}
	if cfg.TelcoDir() != "rag-data/telco-reference" {
		t.Errorf("expected TelcoDir=rag-data/telco-reference, got %s", cfg.TelcoDir())
	}
}

func TestLoadConfigMissing(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("expected no error for missing config, got %v", err)
	}
	if cfg.DataDir != "rag-data" {
		t.Errorf("expected default DataDir, got %s", cfg.DataDir)
	}
}

func TestExtractKeywords(t *testing.T) {
	keywords := extractKeywords("How to troubleshoot etcd on OpenShift cluster", 4)
	if len(keywords) == 0 {
		t.Fatal("expected keywords, got none")
	}

	found := false
	for _, k := range keywords {
		if k == "troubleshoot" || k == "etcd" || k == "openshift" || k == "cluster" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected relevant keywords, got %v", keywords)
	}

	for _, k := range keywords {
		if k == "how" || k == "the" || k == "on" {
			t.Errorf("stop word %q should have been filtered", k)
		}
	}
}

func TestTruncate(t *testing.T) {
	short := "hello"
	if truncate(short, 10) != "hello" {
		t.Errorf("short string should not be truncated")
	}

	long := "hello world this is a long string"
	result := truncate(long, 10)
	if len(result) != 13 { // 10 + "..."
		t.Errorf("expected truncated length 13, got %d", len(result))
	}
}

func TestMetaOrDefault(t *testing.T) {
	m := map[string]string{"key": "value"}
	if metaOrDefault(m, "key", "default") != "value" {
		t.Error("expected existing key to return value")
	}
	if metaOrDefault(m, "missing", "default") != "default" {
		t.Error("expected missing key to return default")
	}
	if metaOrDefault(nil, "key", "default") != "default" {
		t.Error("expected nil map to return default")
	}
}

func TestComputeConfidence(t *testing.T) {
	if computeConfidence(0, 0) != 0 {
		t.Error("zero docs should give zero confidence")
	}
	if computeConfidence(1, 0) < 0.2 {
		t.Error("some docs should give some confidence")
	}
	if computeConfidence(10, 2) <= computeConfidence(1, 0) {
		t.Error("more docs and issues should give higher confidence")
	}
}

func TestBuildTroubleshootQuery(t *testing.T) {
	query := buildTroubleshootQuery("etcd", []string{"pod crashloop", "leader election failed"}, "4.22")
	if query == "" {
		t.Error("expected non-empty query")
	}
}

func TestCollectionConstants(t *testing.T) {
	if len(AllCollections) != 5 {
		t.Errorf("expected 5 collections, got %d", len(AllCollections))
	}
}

func TestFreshnessCheckMissing(t *testing.T) {
	status, err := CheckFreshness(t.TempDir(), ".ingest_meta.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Fresh {
		t.Error("expected not fresh for missing meta")
	}
}
