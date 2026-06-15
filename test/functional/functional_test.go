package functional_test

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"

	"github.com/midu16/opm-troubleshooting/internal/catalog"
	"github.com/midu16/opm-troubleshooting/internal/cli"
	"github.com/midu16/opm-troubleshooting/internal/imageinspect"
)

func TestEndToEnd_fixtureCatalogAndLocalRegistry(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "catalog", "sample.ndjson")
	cfg, err := catalog.LoadDeclarativeConfigFromNDJSON(fixture)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	bundleName, imageRef, err := catalog.ResolveChannelHead(cfg, "operator-a", "stable")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if bundleName != "operator-a.v0.2.0" {
		t.Fatalf("bundle name: %q", bundleName)
	}

	img, err := random.Image(256, 1)
	if err != nil {
		t.Fatalf("random image: %v", err)
	}
	imgCfg, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	imgCfg.Config.Labels = map[string]string{
		"operators.operatorframework.io.bundle.package.v1": "operator-a",
		"version":                       "0.2.0",
		"io.openshift.build.commit.id":  "commit-sha",
		"io.openshift.build.commit.url": "https://example/commit",
	}
	img, err = mutate.ConfigFile(img, imgCfg)
	if err != nil {
		t.Fatalf("mutate: %v", err)
	}

	localRef := imageinspect.StartTestRegistry(t, "operator-a:v0.2.0", img)
	info, err := imageinspect.InspectTestBundle(context.Background(), localRef)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}

	if info.Package != "operator-a" {
		t.Errorf("package: %q", info.Package)
	}
	if info.Version != "0.2.0" {
		t.Errorf("version: %q", info.Version)
	}

	var buf bytes.Buffer
	if err := writeHumanOutput(&buf, info); err != nil {
		t.Fatalf("output: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "package: operator-a") {
		t.Errorf("unexpected output: %s", out)
	}

	// Ensure original catalog resolution still points at expected upstream ref.
	if imageRef != "quay.io/example/operator-a:v0.2.0" {
		t.Errorf("catalog image ref: %q", imageRef)
	}
}

func TestCLI_missingFlags(t *testing.T) {
	err := cli.Run([]string{"--catalog", "img"})
	if err == nil {
		t.Fatal("expected error")
	}
	if cli.ExitCode(err) != 1 {
		t.Errorf("exit code: got %d want 1", cli.ExitCode(err))
	}
}

func writeHumanOutput(w *bytes.Buffer, info *imageinspect.BundleInfo) error {
	lines := []string{
		"package: " + info.Package,
		"bundle:  " + info.Bundle,
		"version: " + info.Version,
		"commit:  " + info.Commit,
		"url:     " + info.URL,
	}
	_, err := w.WriteString(strings.Join(lines, "\n") + "\n")
	return err
}
