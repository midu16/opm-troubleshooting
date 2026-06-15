package imageinspect

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// StartTestRegistry pushes an image to a local test registry and returns its reference.
func StartTestRegistry(t *testing.T, repo string, img v1.Image) string {
	return startTestRegistry(t, repo, img)
}

// StartMultiImageRegistry pushes multiple images to one local test registry and returns the host (host:port).
func StartMultiImageRegistry(t *testing.T, images map[string]v1.Image) string {
	t.Helper()

	s := httptest.NewServer(registry.New())
	t.Cleanup(s.Close)

	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	for repo, img := range images {
		dst := fmt.Sprintf("%s/%s", u.Host, repo)
		ref, err := name.ParseReference(dst, name.Insecure)
		if err != nil {
			t.Fatalf("parse reference %q: %v", dst, err)
		}
		if err := remote.Write(ref, img, remote.WithTransport(http.DefaultTransport)); err != nil {
			t.Fatalf("write image %q: %v", dst, err)
		}
	}

	return u.Host
}

// InspectTestBundle inspects an image in a local test registry (no registry auth).
func InspectTestBundle(ctx context.Context, imageRef string) (*BundleInfo, error) {
	return inspectTestBundle(ctx, imageRef)
}

func startTestRegistry(t *testing.T, repo string, img v1.Image) string {
	t.Helper()

	s := httptest.NewServer(registry.New())
	t.Cleanup(s.Close)

	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	dst := fmt.Sprintf("%s/%s", u.Host, repo)
	ref, err := name.ParseReference(dst, name.Insecure)
	if err != nil {
		t.Fatalf("parse reference: %v", err)
	}

	if err := remote.Write(ref, img, remote.WithTransport(http.DefaultTransport)); err != nil {
		t.Fatalf("write image: %v", err)
	}

	return dst
}

func inspectTestBundle(ctx context.Context, imageRef string) (*BundleInfo, error) {
	return inspectBundle(ctx, imageRef, remote.WithTransport(http.DefaultTransport))
}
