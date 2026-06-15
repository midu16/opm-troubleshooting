//go:build integration

package integration_test

import (
	"os"
	"testing"
	"time"

	"github.com/midu16/opm-troubleshooting/internal/cli"
)

func TestLiveCatalogInspect(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("set RUN_INTEGRATION_TESTS=1 to run integration tests")
	}
	if os.Getenv("DOCKER_CONFIG") == "" && os.Getenv("REGISTRY_AUTH_FILE") == "" {
		t.Skip("set DOCKER_CONFIG or REGISTRY_AUTH_FILE for registry authentication")
	}

	catalog := envOr("TEST_CATALOG", "quay.io/prega/prega-operator-index:v4.22-20260607T194312")
	pkg := envOr("TEST_PACKAGE", "kubernetes-nmstate-operator")
	channel := envOr("TEST_CHANNEL", "stable")

	args := []string{
		"--catalog", catalog,
		"--package", pkg,
		"--channel", channel,
		"--timeout", "15m",
	}

	done := make(chan error, 1)
	go func() {
		done <- cli.Run(args)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("cli run failed: %v", err)
		}
	case <-time.After(16 * time.Minute):
		t.Fatal("integration test timed out")
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
