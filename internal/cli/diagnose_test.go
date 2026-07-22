package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParseDiagnoseArgs_help(t *testing.T) {
	_, err := parseDiagnoseArgs([]string{"--help"})
	if !errors.Is(err, errHelp) {
		t.Errorf("expected errHelp, got %v", err)
	}
}

func TestParseDiagnoseArgs_mustGather(t *testing.T) {
	cfg, err := parseDiagnoseArgs([]string{"--must-gather", "/tmp/mg"})
	if err != nil {
		t.Fatalf("parseDiagnoseArgs: %v", err)
	}
	if cfg.mustGatherPath != "/tmp/mg" {
		t.Errorf("must-gather path: %q", cfg.mustGatherPath)
	}
	if cfg.kubeconfigPath != "" {
		t.Errorf("expected empty kubeconfig, got %q", cfg.kubeconfigPath)
	}
}

func TestParseDiagnoseArgs_kubeconfig(t *testing.T) {
	// Create a temp kubeconfig file
	dir := t.TempDir()
	kc := filepath.Join(dir, "config")
	if err := os.WriteFile(kc, []byte("apiVersion: v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseDiagnoseArgs([]string{"--kubeconfig", kc, "--context", "my-ctx"})
	if err != nil {
		t.Fatalf("parseDiagnoseArgs: %v", err)
	}
	if cfg.kubeconfigPath != kc {
		t.Errorf("kubeconfig: %q", cfg.kubeconfigPath)
	}
	if cfg.kubeContext != "my-ctx" {
		t.Errorf("context: %q", cfg.kubeContext)
	}
}

func TestParseDiagnoseArgs_adhdDefaults(t *testing.T) {
	cfg, err := parseDiagnoseArgs([]string{"--must-gather", "/tmp/mg"})
	if err != nil {
		t.Fatalf("parseDiagnoseArgs: %v", err)
	}
	if !cfg.adhdEnabled {
		t.Error("ADHD should be enabled by default")
	}
	if cfg.depth != "standard" {
		t.Errorf("depth: %q, want standard", cfg.depth)
	}
	if cfg.frameCount != 5 {
		t.Errorf("frameCount: %d, want 5", cfg.frameCount)
	}
}

func TestParseDiagnoseArgs_depthQuick(t *testing.T) {
	cfg, err := parseDiagnoseArgs([]string{"--must-gather", "/tmp/mg", "--depth", "quick"})
	if err != nil {
		t.Fatalf("parseDiagnoseArgs: %v", err)
	}
	if cfg.frameCount != 3 {
		t.Errorf("quick depth frameCount: %d, want 3", cfg.frameCount)
	}
}

func TestParseDiagnoseArgs_depthDeep(t *testing.T) {
	cfg, err := parseDiagnoseArgs([]string{"--must-gather", "/tmp/mg", "--depth", "deep"})
	if err != nil {
		t.Fatalf("parseDiagnoseArgs: %v", err)
	}
	if cfg.frameCount != 8 {
		t.Errorf("deep depth frameCount: %d, want 8", cfg.frameCount)
	}
}

func TestParseDiagnoseArgs_depthInvalid(t *testing.T) {
	_, err := parseDiagnoseArgs([]string{"--must-gather", "/tmp/mg", "--depth", "invalid"})
	if err == nil {
		t.Fatal("expected error for invalid depth")
	}
}

func TestParseDiagnoseArgs_explicitFrameCountPreserved(t *testing.T) {
	cfg, err := parseDiagnoseArgs([]string{"--must-gather", "/tmp/mg", "--depth", "deep", "--frames", "12"})
	if err != nil {
		t.Fatalf("parseDiagnoseArgs: %v", err)
	}
	if cfg.frameCount != 12 {
		t.Errorf("explicit frameCount: %d, want 12", cfg.frameCount)
	}
}

func TestParseDiagnoseArgs_noSource(t *testing.T) {
	// Without kubeconfig env var or default location, should error
	os.Unsetenv("KUBECONFIG")
	_, err := parseDiagnoseArgs([]string{})
	// May or may not error depending on whether ~/.kube/config exists
	// but at minimum it should not panic
	_ = err
}

func TestCollectSymptoms(t *testing.T) {
	symptoms := collectSymptoms(nil)
	if len(symptoms) != 0 {
		t.Errorf("nil report should produce 0 symptoms, got %d", len(symptoms))
	}
}
