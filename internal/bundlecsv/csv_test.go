package bundlecsv

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

func TestRepositoryURLsFromAnnotations(t *testing.T) {
	ann := map[string]string{
		"repository": "https://github.com/red-hat-storage/odf-operator",
		"other":      "see https://github.com/example/foo for details",
	}
	got := repositoryURLsFromAnnotations(ann)
	if len(got) < 1 || got[0] != "https://github.com/red-hat-storage/odf-operator" {
		t.Fatalf("got %v", got)
	}
}

func TestRepositoryURLsFromBundleLayer(t *testing.T) {
	csv := []byte(`apiVersion: operators.coreos.com/v1alpha1
kind: ClusterServiceVersion
metadata:
  name: odf-operator.v4.21.7
  annotations:
    repository: https://github.com/red-hat-storage/odf-operator
`)

	layerData := tarGzLayer(map[string][]byte{
		"manifests/odf-operator.clusterserviceversion.yaml": csv,
	})

	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(layerData)), nil
	})
	if err != nil {
		t.Fatalf("layer: %v", err)
	}

	img, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		t.Fatalf("append: %v", err)
	}

	repos, err := RepositoryURLs(img, "odf-operator")
	if err != nil {
		t.Fatalf("RepositoryURLs: %v", err)
	}
	if len(repos) != 1 || repos[0] != "https://github.com/red-hat-storage/odf-operator" {
		t.Fatalf("got %v", repos)
	}
}

func TestNormalizeGitRepoURL(t *testing.T) {
	tests := map[string]string{
		"github.com/cryostatio/cryostat-operator":         "https://github.com/cryostatio/cryostat-operator",
		"https://github.com/foo/bar/":                     "https://github.com/foo/bar",
		"https://catalog.redhat.com/en":                   "",
		"https://github.com/red-hat-storage/odf-operator": "https://github.com/red-hat-storage/odf-operator",
	}
	for in, want := range tests {
		if got := normalizeGitRepoURL(in); got != want {
			t.Errorf("%q => %q want %q", in, got, want)
		}
	}
}

func TestRepositoryURLsFromSpecLinks(t *testing.T) {
	doc := map[string]interface{}{
		"spec": map[string]interface{}{
			"links": []interface{}{
				map[string]interface{}{
					"name": "Source Code",
					"url":  "https://github.com/red-hat-storage/odf-operator",
				},
				map[string]interface{}{
					"name": "Product Page",
					"url":  "https://access.redhat.com/products/foo",
				},
			},
		},
	}
	got := repositoryURLsFromDocument(doc, "")
	if len(got) != 1 || got[0] != "https://github.com/red-hat-storage/odf-operator" {
		t.Fatalf("got %v", got)
	}
}

func TestRepositoryURLsFromAnnotationBareGitHub(t *testing.T) {
	doc := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"repository": "github.com/cryostatio/cryostat-operator",
			},
		},
	}
	got := repositoryURLsFromDocument(doc, "")
	if len(got) != 1 || got[0] != "https://github.com/cryostatio/cryostat-operator" {
		t.Fatalf("got %v", got)
	}
}

func TestRepositoryURLsFromDevspacesCSVPath(t *testing.T) {
	csv := []byte(`apiVersion: operators.coreos.com/v1alpha1
kind: ClusterServiceVersion
metadata:
  name: devspaces.v3.28.0
  annotations:
    repository: https://github.com/redhat-developer/devspaces-images/
spec:
  links: []
`)

	layerData := tarGzLayer(map[string][]byte{
		"manifests/devspaces.csv.yaml": csv,
	})

	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(layerData)), nil
	})
	if err != nil {
		t.Fatal(err)
	}

	img, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		t.Fatal(err)
	}
	repos, err := RepositoryURLs(img, "devspaces")
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0] != "https://github.com/redhat-developer/devspaces-images" {
		t.Fatalf("got %v", repos)
	}
}

func repositoryURLsFromAnnotations(ann map[string]string) []string {
	doc := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{},
		},
	}
	raw := doc["metadata"].(map[string]interface{})["annotations"].(map[string]interface{})
	for k, v := range ann {
		raw[k] = v
	}
	return repositoryURLsFromDocument(doc, "")
}

func tarGzLayer(files map[string][]byte) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}); err != nil {
			panic(err)
		}
		if _, err := tw.Write(content); err != nil {
			panic(err)
		}
	}
	if err := tw.Close(); err != nil {
		panic(err)
	}
	if err := gz.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}
