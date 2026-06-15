package catalog

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveChannelVersion(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "catalog", "sample.ndjson")
	cfg, err := LoadDeclarativeConfigFromNDJSON(fixture)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	bundleName, imageRef, err := ResolveChannelVersion(cfg, "operator-a", "stable", "0.1.0")
	if err != nil {
		t.Fatalf("ResolveChannelVersion: %v", err)
	}
	if bundleName != "operator-a.v0.1.0" {
		t.Fatalf("bundle: %q", bundleName)
	}
	if imageRef != "quay.io/example/operator-a:v0.1.0" {
		t.Fatalf("image: %q", imageRef)
	}

	bundleName, imageRef, err = ResolveChannelVersion(cfg, "operator-a", "stable", "v0.2.0")
	if err != nil {
		t.Fatalf("ResolveChannelVersion head prefix: %v", err)
	}
	if bundleName != "operator-a.v0.2.0" {
		t.Fatalf("bundle: %q", bundleName)
	}
	if imageRef != "quay.io/example/operator-a:v0.2.0" {
		t.Fatalf("image: %q", imageRef)
	}
}

func TestResolveChannelVersion_notFound(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "catalog", "sample.ndjson")
	cfg, err := LoadDeclarativeConfigFromNDJSON(fixture)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	_, _, err = ResolveChannelVersion(cfg, "operator-a", "stable", "9.9.9")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "version \"9.9.9\" not found") {
		t.Errorf("message: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "versions on channel") {
		t.Errorf("message: %q", err.Error())
	}
}

func TestVersionMatches(t *testing.T) {
	tests := []struct {
		query, bundle string
		want          bool
	}{
		{"v2.11.2", "2.11.2-509", true},
		{"2.11.2", "v2.11.2-509", true},
		{"v2.11.1", "2.11.1-508", true},
		{"2.11.2-509", "2.11.2-509", true},
		{"2.11.2", "2.11.1-508", false},
		{"0.1.0", "0.1.0", true},
	}
	for _, tt := range tests {
		if got := versionMatches(tt.query, tt.bundle); got != tt.want {
			t.Errorf("versionMatches(%q, %q) = %v want %v", tt.query, tt.bundle, got, tt.want)
		}
	}
}
