package imageinspect

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
)

func TestInspectBundle(t *testing.T) {
	img, err := random.Image(256, 1)
	if err != nil {
		t.Fatalf("random image: %v", err)
	}

	cfg, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("config file: %v", err)
	}
	cfg.Config.Labels = map[string]string{
		labelBundlePackage:  "test-operator",
		labelVersion:        "1.2.3",
		labelBuildCommitID:  "abc123",
		labelBuildCommitURL: "https://github.com/example/commit/abc123",
	}
	img, err = mutate.ConfigFile(img, cfg)
	if err != nil {
		t.Fatalf("mutate config: %v", err)
	}

	imageRef := startTestRegistry(t, "test/bundle:v1", img)
	info, err := inspectTestBundle(context.Background(), imageRef)
	if err != nil {
		t.Fatalf("inspect bundle: %v", err)
	}

	if info.Package != "test-operator" {
		t.Errorf("package: got %q want test-operator", info.Package)
	}
	if info.Version != "1.2.3" {
		t.Errorf("version: got %q want 1.2.3", info.Version)
	}
	if info.Commit != "abc123" {
		t.Errorf("commit: got %q want abc123", info.Commit)
	}
	if info.URL != "https://github.com/example/commit/abc123" {
		t.Errorf("url: got %q", info.URL)
	}
	if info.Bundle == "" {
		t.Error("expected non-empty bundle display name")
	}
}

func TestFirstLabel_commitFallbacks(t *testing.T) {
	labels := map[string]string{
		labelVCSRef:      "vcs-only",
		labelOCIRevision: "oci-revision",
	}
	if got := firstLabel(labels, commitLabelKeys); got != "oci-revision" {
		t.Errorf("got %q want oci-revision", got)
	}

	labels = map[string]string{
		labelBuildCommitID: "openshift-commit",
		labelVCSRef:        "vcs-only",
	}
	if got := firstLabel(labels, commitLabelKeys); got != "openshift-commit" {
		t.Errorf("got %q want openshift-commit", got)
	}
}

func TestInspectBundle_redHatStyleLabels(t *testing.T) {
	img, err := random.Image(256, 1)
	if err != nil {
		t.Fatalf("random image: %v", err)
	}

	cfg, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("config file: %v", err)
	}
	cfg.Config.Labels = map[string]string{
		labelBundlePackage: "odf-operator",
		labelVersion:       "v4.21.7",
		labelVCSRef:        "5d1e5bd5b55d8ba2fd2a2c1199ff819e9537ea25",
		labelOCIRevision:   "5d1e5bd5b55d8ba2fd2a2c1199ff819e9537ea25",
	}
	img, err = mutate.ConfigFile(img, cfg)
	if err != nil {
		t.Fatalf("mutate config: %v", err)
	}

	imageRef := startTestRegistry(t, "odf/bundle:v4.21.7", img)
	info, err := inspectTestBundle(context.Background(), imageRef)
	if err != nil {
		t.Fatalf("inspect bundle: %v", err)
	}
	if info.Commit != "5d1e5bd5b55d8ba2fd2a2c1199ff819e9537ea25" {
		t.Errorf("commit: got %q", info.Commit)
	}
	if info.URL != "" {
		t.Errorf("url: got %q want empty for Red Hat-style labels without repo URL", info.URL)
	}
}

func TestDeriveGitCommitURL_github(t *testing.T) {
	want := "https://github.com/stolostron/acm-operator-bundle/commit/18a4800b6c7fb8fc611e59d4f508315c1e4010d0"
	if got := deriveGitCommitURLFromRepo("https://github.com/stolostron/acm-operator-bundle", "18a4800b6c7fb8fc611e59d4f508315c1e4010d0", "git"); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestDeriveGitCommitURL_skipsCatalogURL(t *testing.T) {
	if got := deriveGitCommitURLFromRepo("https://catalog.redhat.com/en", "abc123", "git"); got != "" {
		t.Errorf("got %q want empty", got)
	}
}

func TestInspectBundle_acmStyleLabels(t *testing.T) {
	img, err := random.Image(256, 1)
	if err != nil {
		t.Fatalf("random image: %v", err)
	}

	cfg, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("config file: %v", err)
	}
	cfg.Config.Labels = map[string]string{
		labelBundlePackage: "advanced-cluster-management",
		labelVersion:       "2.16.2-454",
		labelVCSRef:        "18a4800b6c7fb8fc611e59d4f508315c1e4010d0",
		labelOCISource:     "https://github.com/stolostron/acm-operator-bundle",
		labelVCSType:       "git",
	}
	img, err = mutate.ConfigFile(img, cfg)
	if err != nil {
		t.Fatalf("mutate config: %v", err)
	}

	imageRef := startTestRegistry(t, "rhacm2/acm-operator-bundle:2.16.2-454", img)
	info, err := inspectTestBundle(context.Background(), imageRef)
	if err != nil {
		t.Fatalf("inspect bundle: %v", err)
	}
	if info.Commit != "18a4800b6c7fb8fc611e59d4f508315c1e4010d0" {
		t.Errorf("commit: got %q", info.Commit)
	}
	wantURL := "https://github.com/stolostron/acm-operator-bundle/commit/18a4800b6c7fb8fc611e59d4f508315c1e4010d0"
	if info.URL != wantURL {
		t.Errorf("url: got %q want %q", info.URL, wantURL)
	}
}

func TestInspectBundle_invalidReference(t *testing.T) {
	_, err := InspectBundle(context.Background(), "not-a-valid-ref")
	if err == nil {
		t.Fatal("expected error for invalid reference")
	}
}

func TestBundleDisplayName_tagged(t *testing.T) {
	ref, err := name.ParseReference("quay.io/example/bundle:tag", name.StrictValidation)
	if err != nil {
		t.Fatalf("parse ref: %v", err)
	}
	name, err := bundleDisplayName(ref, empty.Image)
	if err != nil {
		t.Fatalf("bundle display name: %v", err)
	}
	if name != "quay.io/example/bundle:tag" {
		t.Errorf("got %q", name)
	}
}

func TestBundleDisplayName_digest(t *testing.T) {
	img, err := random.Image(128, 1)
	if err != nil {
		t.Fatalf("random image: %v", err)
	}
	digest, err := img.Digest()
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	ref, err := name.ParseReference("quay.io/example/bundle-repo@"+digest.String(), name.Insecure)
	if err != nil {
		t.Fatalf("parse ref: %v", err)
	}
	name, err := bundleDisplayName(ref, img)
	if err != nil {
		t.Fatalf("bundle display name: %v", err)
	}
	if !strings.Contains(name, "quay.io/example/bundle-repo@sha256:") {
		t.Errorf("expected digest reference, got %q", name)
	}
}

// Ensure bundleDisplayName accepts v1.Image interface.
var _ v1.Image = empty.Image
