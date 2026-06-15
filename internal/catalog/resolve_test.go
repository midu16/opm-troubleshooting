package catalog

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
)

func TestResolveChannelHead(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "catalog", "sample.ndjson")
	cfg, err := LoadDeclarativeConfigFromNDJSON(fixture)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	tests := []struct {
		name        string
		packageName string
		channelName string
		wantBundle  string
		wantImage   string
		wantErr     bool
	}{
		{
			name:        "channel head is last entry",
			packageName: "operator-a",
			channelName: "stable",
			wantBundle:  "operator-a.v0.2.0",
			wantImage:   "quay.io/example/operator-a:v0.2.0",
		},
		{
			name:        "other package channel",
			packageName: "operator-b",
			channelName: "beta",
			wantBundle:  "operator-b.v1.0.0",
			wantImage:   "quay.io/example/operator-b:v1.0.0",
		},
		{
			name:        "missing channel",
			packageName: "operator-a",
			channelName: "beta",
			wantErr:     true,
		},
		{
			name:        "empty channel entries",
			packageName: "operator-b",
			channelName: "stable",
			wantErr:     true,
		},
		{
			name:        "missing package",
			packageName: "missing-operator",
			channelName: "stable",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundleName, imageRef, err := ResolveChannelHead(cfg, tt.packageName, tt.channelName)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got bundle=%s image=%s", bundleName, imageRef)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if bundleName != tt.wantBundle {
				t.Errorf("bundle name: got %q want %q", bundleName, tt.wantBundle)
			}
			if imageRef != tt.wantImage {
				t.Errorf("image ref: got %q want %q", imageRef, tt.wantImage)
			}
		})
	}
}

func TestChannelsForPackage(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "catalog", "sample.ndjson")
	cfg, err := LoadDeclarativeConfigFromNDJSON(fixture)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	got := ChannelsForPackage(cfg, "operator-a")
	if len(got) != 1 || got[0] != "stable" {
		t.Fatalf("operator-a channels: %v", got)
	}

	got = ChannelsForPackage(cfg, "operator-b")
	if len(got) != 2 {
		t.Fatalf("operator-b channels: %v", got)
	}
}

func TestResolveChannelHead_missingChannelListsAvailable(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "catalog", "sample.ndjson")
	cfg, err := LoadDeclarativeConfigFromNDJSON(fixture)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	_, _, err = ResolveChannelHead(cfg, "operator-a", "beta")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "channel \"beta\" not found") {
		t.Errorf("message: %q", msg)
	}
	if !strings.Contains(msg, "defaultChannel: stable") {
		t.Errorf("message: %q", msg)
	}
	if !strings.Contains(msg, "available channels: stable") {
		t.Errorf("message: %q", msg)
	}
}

func TestResolveChannelHead_nilConfig(t *testing.T) {
	_, _, err := ResolveChannelHead(nil, "pkg", "stable")
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestResolveChannelHead_missingBundleRecord(t *testing.T) {
	cfg := &declcfg.DeclarativeConfig{
		Channels: []declcfg.Channel{
			{
				Schema:  declcfg.SchemaChannel,
				Name:    "stable",
				Package: "operator-a",
				Entries: []declcfg.ChannelEntry{{Name: "operator-a.v9.9.9"}},
			},
		},
	}
	_, _, err := ResolveChannelHead(cfg, "operator-a", "stable")
	if err == nil {
		t.Fatal("expected error when bundle record is missing")
	}
}

func TestResolveChannelHead_emptyImage(t *testing.T) {
	cfg := &declcfg.DeclarativeConfig{
		Channels: []declcfg.Channel{
			{
				Schema:  declcfg.SchemaChannel,
				Name:    "stable",
				Package: "operator-a",
				Entries: []declcfg.ChannelEntry{{Name: "operator-a.v0.3.0"}},
			},
		},
		Bundles: []declcfg.Bundle{
			{
				Schema:  declcfg.SchemaBundle,
				Name:    "operator-a.v0.3.0",
				Package: "operator-a",
				Image:   "",
			},
		},
	}
	_, _, err := ResolveChannelHead(cfg, "operator-a", "stable")
	if err == nil {
		t.Fatal("expected error when bundle image is empty")
	}
}
