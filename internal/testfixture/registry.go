package testfixture

import (
	"context"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"

	"github.com/midu16/opm-troubleshooting/internal/imageinspect"
)

// BuildBundleImage creates a bundle image with standard OLM labels for mock registry tests.
func BuildBundleImage(spec BundleImageSpec) (v1.Image, error) {
	img, err := random.Image(256, 1)
	if err != nil {
		return nil, err
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, err
	}
	cfg.Config.Labels = map[string]string{
		"operators.operatorframework.io.bundle.package.v1": spec.Package,
		"version":                       spec.Version,
		"io.openshift.build.commit.id":  spec.Commit,
		"io.openshift.build.commit.url": spec.URL,
	}
	return mutate.ConfigFile(img, cfg)
}

// StartMockRegistry pushes bundle images and returns the registry host (host:port).
func StartMockRegistry(t *testing.T, specs []BundleImageSpec) string {
	t.Helper()
	images := make(map[string]v1.Image, len(specs))
	for _, spec := range specs {
		img, err := BuildBundleImage(spec)
		if err != nil {
			t.Fatalf("build image %s: %v", spec.RepoPath, err)
		}
		images[spec.RepoPath] = img
	}
	return imageinspect.StartMultiImageRegistry(t, images)
}

// InspectFromMockRegistry inspects a bundle image reference on the local test registry.
func InspectFromMockRegistry(ctx context.Context, registryHost, repoPath string) (*imageinspect.BundleInfo, error) {
	return imageinspect.InspectTestBundle(ctx, registryHost+"/"+repoPath)
}
