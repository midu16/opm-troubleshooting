package workflow

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/midu16/opm-troubleshooting/internal/catalog"
)

func TestInspectChannelHeadFromConfig_fixture(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "catalog", "sample.ndjson")
	cfg, err := catalog.LoadDeclarativeConfigFromNDJSON(fixture)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	_, err = InspectChannelHeadFromConfig(context.Background(), cfg, "operator-a", "stable")
	if err == nil {
		t.Fatal("expected inspect error without reachable bundle image")
	}
}

func TestResolveDefaultChannel(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "catalog", "sample.ndjson")
	cfg, err := catalog.LoadDeclarativeConfigFromNDJSON(fixture)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	ch, err := ResolveDefaultChannel(cfg, "operator-a")
	if err != nil {
		t.Fatalf("ResolveDefaultChannel: %v", err)
	}
	if ch != "stable" {
		t.Fatalf("got %q want stable", ch)
	}

	_, err = ResolveDefaultChannel(cfg, "missing")
	if err == nil {
		t.Fatal("expected error for missing package")
	}
}
