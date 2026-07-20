package cli

import (
	"testing"
)

func TestParseTelcoArgs_requiresMustGather(t *testing.T) {
	_, err := parseTelcoArgs([]string{"--catalog", "quay.io/test:index"})
	if err == nil {
		t.Fatal("expected error without --must-gather")
	}
}

func TestParseTelcoArgs_defaults(t *testing.T) {
	cfg, err := parseTelcoArgs([]string{
		"--must-gather", "/tmp/mg",
		"--environment", "lab",
		"--rca-file", "/tmp/rca.md",
	})
	if err != nil {
		t.Fatalf("parseTelcoArgs: %v", err)
	}
	if cfg.mustGatherPath != "/tmp/mg" {
		t.Errorf("must-gather: %q", cfg.mustGatherPath)
	}
	if cfg.environment != "lab" {
		t.Errorf("environment: %q", cfg.environment)
	}
	if cfg.rcaFile != "/tmp/rca.md" {
		t.Errorf("rca-file: %q", cfg.rcaFile)
	}
	if cfg.skipRCA {
		t.Error("RCA should be enabled by default")
	}
	if cfg.skipHealth {
		t.Error("health check should be enabled by default")
	}
}

func TestValidateTelcoEnvironment(t *testing.T) {
	for _, env := range []string{"production", "disconnected", "lab", "kvm"} {
		if !ValidateTelcoEnvironment(env) {
			t.Errorf("expected valid environment %q", env)
		}
	}
}
