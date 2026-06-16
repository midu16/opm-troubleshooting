package bundlecsv

import (
	"archive/tar"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"sigs.k8s.io/yaml"
)

var (
	githubRepoURL     = regexp.MustCompile(`https?://github\.com/[\w.-]+/[\w.-]+`)
	bareGitHubRepoURL = regexp.MustCompile(`(?:^|[^\w/])(github\.com/[\w.-]+/[\w.-]+)`)
	gitlabRepoURL     = regexp.MustCompile(`https?://gitlab\.com/[\w.-]+/[\w.-]+`)
)

const (
	annotationRepository          = "repository"
	annotationFrameworkRepository = "operators.operatorframework.io/repository"
	annotationFrameworkSource     = "operators.operatorframework.io/source"
	annotationSource              = "source"
)

var repositoryAnnotationKeys = []string{
	annotationRepository,
	annotationFrameworkRepository,
	annotationFrameworkSource,
	annotationSource,
}

// RepositoryURLs returns operator source repository URLs from bundle CSV content and annotations.
func RepositoryURLs(img v1.Image, packageName string) ([]string, error) {
	ordered := make([]string, 0, 4)
	seen := make(map[string]struct{})

	addURL := func(u string) {
		normalized := normalizeGitRepoURL(u)
		if normalized == "" {
			return
		}
		if _, ok := seen[normalized]; ok {
			return
		}
		seen[normalized] = struct{}{}
		ordered = append(ordered, normalized)
	}

	doc, docErr := CSVDocument(img)
	if docErr == nil {
		for _, u := range repositoryURLsFromDocument(doc, packageName) {
			addURL(u)
		}
	}

	layerURLs, err := repositoryURLsFromManifestLayers(img)
	if err != nil {
		if docErr != nil {
			return nil, docErr
		}
		return ordered, nil
	}
	for _, u := range layerURLs {
		addURL(u)
	}

	if len(ordered) == 0 && docErr != nil {
		return nil, docErr
	}
	if len(ordered) == 0 {
		for _, u := range inferRepositoryURLs(packageName, nil) {
			addURL(u)
		}
	}
	return ordered, nil
}

// CSVAnnotations reads metadata.annotations from the first CSV in the bundle image.
func CSVAnnotations(img v1.Image) (map[string]string, error) {
	doc, err := CSVDocument(img)
	if err != nil {
		return nil, err
	}
	return annotationsFromDocument(doc), nil
}

// CSVDocument reads and parses the first ClusterServiceVersion manifest in the bundle.
func CSVDocument(img v1.Image) (map[string]interface{}, error) {
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("list layers: %w", err)
	}

	for _, layer := range layers {
		doc, found, err := csvDocumentFromLayer(layer)
		if err != nil {
			return nil, err
		}
		if found {
			return doc, nil
		}
	}
	return nil, fmt.Errorf("clusterserviceversion not found in bundle image")
}

func csvDocumentFromLayer(layer v1.Layer) (map[string]interface{}, bool, error) {
	rc, err := layer.Uncompressed()
	if err != nil {
		return nil, false, fmt.Errorf("open layer: %w", err)
	}
	defer rc.Close()

	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, fmt.Errorf("read tar: %w", err)
		}
		if !isCSVPath(hdr.Name) {
			continue
		}

		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, false, fmt.Errorf("read csv %q: %w", hdr.Name, err)
		}

		var doc map[string]interface{}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, false, fmt.Errorf("parse csv %q: %w", hdr.Name, err)
		}
		return doc, true, nil
	}
}

func isCSVPath(path string) bool {
	if !strings.Contains(path, "manifests/") || !strings.HasSuffix(path, ".yaml") {
		return false
	}
	lower := strings.ToLower(path)
	return strings.Contains(lower, "clusterserviceversion") || strings.HasSuffix(lower, ".csv.yaml")
}

func annotationsFromDocument(doc map[string]interface{}) map[string]string {
	meta, _ := doc["metadata"].(map[string]interface{})
	if meta == nil {
		return nil
	}
	raw, _ := meta["annotations"].(map[string]interface{})
	ann := make(map[string]string, len(raw))
	for k, v := range raw {
		if s, ok := v.(string); ok {
			ann[k] = s
		}
	}
	return ann
}

func repositoryURLsFromDocument(doc map[string]interface{}, packageName string) []string {
	ordered := make([]string, 0, 4)
	seen := make(map[string]struct{})

	add := func(val string) {
		for _, u := range extractGitRepoURLs(val) {
			normalized := normalizeGitRepoURL(u)
			if normalized == "" {
				continue
			}
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			ordered = append(ordered, normalized)
		}
	}

	ann := annotationsFromDocument(doc)
	for _, key := range repositoryAnnotationKeys {
		add(ann[key])
	}
	for key, val := range ann {
		if containsRepositoryKey(key) {
			continue
		}
		if strings.Contains(strings.ToLower(val), "github.com") || strings.Contains(strings.ToLower(val), "gitlab.com") {
			add(val)
		}
	}

	for _, u := range repositoryURLsFromSpecLinks(doc) {
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		ordered = append(ordered, u)
	}

	if spec, ok := doc["spec"].(map[string]interface{}); ok {
		if desc, ok := spec["description"].(string); ok {
			add(desc)
		}
	}

	for _, u := range inferRepositoryURLs(packageName, ann) {
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		ordered = append(ordered, u)
	}

	return ordered
}

func repositoryURLsFromManifestLayers(img v1.Image) ([]string, error) {
	layers, err := img.Layers()
	if err != nil {
		return nil, err
	}
	ordered := make([]string, 0, 2)
	seen := make(map[string]struct{})
	for _, layer := range layers {
		rc, err := layer.Uncompressed()
		if err != nil {
			continue
		}
		tr := tar.NewReader(rc)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				rc.Close()
				return nil, err
			}
			if !strings.Contains(hdr.Name, "manifests/") || !strings.HasSuffix(hdr.Name, ".yaml") {
				continue
			}
			data, err := io.ReadAll(tr)
			if err != nil {
				rc.Close()
				return nil, err
			}
			for _, u := range extractGitRepoURLs(string(data)) {
				normalized := normalizeGitRepoURL(u)
				if normalized == "" {
					continue
				}
				if _, ok := seen[normalized]; ok {
					continue
				}
				seen[normalized] = struct{}{}
				ordered = append(ordered, normalized)
			}
		}
		rc.Close()
	}
	return ordered, nil
}

type scoredURL struct {
	url   string
	score int
}

func repositoryURLsFromSpecLinks(doc map[string]interface{}) []string {
	spec, ok := doc["spec"].(map[string]interface{})
	if !ok {
		return nil
	}
	raw, ok := spec["links"].([]interface{})
	if !ok {
		return nil
	}

	var scored []scoredURL
	for _, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		url, _ := m["url"].(string)
		normalized := normalizeGitRepoURL(url)
		if normalized == "" {
			normalized = normalizeDocsSiteURL(url)
		}
		if normalized == "" {
			continue
		}
		scored = append(scored, scoredURL{url: normalized, score: scoreSpecLink(name, normalized)})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	out := make([]string, 0, len(scored))
	for _, s := range scored {
		out = append(out, s.url)
	}
	return out
}

func scoreSpecLink(name, url string) int {
	n := strings.ToLower(name)
	u := strings.ToLower(url)
	score := 0
	if strings.Contains(n, "source code") {
		score += 100
	}
	if strings.Contains(n, "github repo") || strings.Contains(n, "github") {
		score += 90
	}
	if strings.Contains(n, "repository") {
		score += 80
	}
	if strings.Contains(n, "operator") && strings.Contains(u, "operator") {
		score += 70
	}
	if strings.Contains(u, "/commit/") {
		score -= 100
	}
	if strings.Contains(u, "graph-data") || strings.Contains(u, "/docs.") {
		score -= 40
	}
	return score
}

func containsRepositoryKey(key string) bool {
	for _, k := range repositoryAnnotationKeys {
		if key == k {
			return true
		}
	}
	return false
}

func extractGitRepoURLs(val string) []string {
	if val == "" {
		return nil
	}
	seen := make(map[string]struct{})
	add := func(u string) {
		u = strings.TrimSuffix(strings.TrimSpace(u), "/")
		if u == "" {
			return
		}
		if _, ok := seen[u]; ok {
			return
		}
		seen[u] = struct{}{}
	}

	for _, m := range githubRepoURL.FindAllString(val, -1) {
		add(m)
	}
	for _, m := range gitlabRepoURL.FindAllString(val, -1) {
		add(m)
	}
	for _, m := range bareGitHubRepoURL.FindAllStringSubmatch(val, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}

	out := make([]string, 0, len(seen))
	for u := range seen {
		out = append(out, u)
	}
	return out
}

func normalizeDocsSiteURL(u string) string {
	lower := strings.ToLower(strings.TrimSpace(u))
	switch {
	case strings.Contains(lower, "csi-addons.github.io"):
		return "https://github.com/csi-addons/kubernetes"
	}
	return ""
}

func inferRepositoryURLs(packageName string, ann map[string]string) []string {
	if ann != nil {
		layout := ann["operators.operatorframework.io/project_layout"]
		if strings.Contains(layout, "ansible") {
			return []string{"https://github.com/ansible/ansible-rulebook"}
		}
	}

	switch packageName {
	case "amq-streams-proxy":
		return []string{"https://github.com/kroxylicious/kroxylicious"}
	case "odf-csi-addons-operator":
		return []string{"https://github.com/csi-addons/kubernetes"}
	case "ansible-automation-platform-operator", "ansible-cloud-addons-operator":
		return []string{"https://github.com/ansible/ansible-rulebook"}
	}
	return nil
}

func normalizeGitRepoURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	if strings.Contains(strings.ToLower(u), "catalog.redhat.com") ||
		strings.Contains(strings.ToLower(u), "access.redhat.com") {
		return ""
	}
	if strings.HasPrefix(u, "github.com/") {
		u = "https://" + u
	}
	if strings.HasPrefix(u, "gitlab.com/") {
		u = "https://" + u
	}
	u = strings.TrimSuffix(u, "/")
	lower := strings.ToLower(u)
	if strings.Contains(lower, "github.com") || strings.Contains(lower, "gitlab.com") {
		return u
	}
	return ""
}
