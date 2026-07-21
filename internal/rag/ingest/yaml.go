package ingest

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/midu16/opm-troubleshooting/internal/rag"
)

var (
	reK8sKind       = regexp.MustCompile(`(?m)^kind:\s*(\S+)`)
	reK8sAPIVersion = regexp.MustCompile(`(?m)^apiVersion:\s*(\S+)`)
	reK8sName       = regexp.MustCompile(`(?m)^\s{2}name:\s*(\S+)`)
)

// LoadYAMLDir walks a directory for *.yaml and *.yml files and returns
// one Document per file (YAML manifests are typically small enough to
// keep as single chunks). Files containing K8s Secrets are filtered by
// FilterAndRedact.
func LoadYAMLDir(dir string) ([]rag.Document, error) {
	var docs []rag.Document

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if info.Name() == "vendor" || info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		// Skip tiny files.
		if info.Size() < 10 {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		content := string(data)

		// Apply secret filter.
		filtered, shouldSkip := FilterAndRedact(path, content)
		if shouldSkip {
			return nil
		}
		content = filtered

		relPath, _ := filepath.Rel(dir, path)

		// Extract K8s metadata via regex.
		kind := extractMatch(reK8sKind, content)
		apiVersion := extractMatch(reK8sAPIVersion, content)
		name := extractMatch(reK8sName, content)

		hash := sha256.Sum256([]byte(relPath + ":" + content))
		id := fmt.Sprintf("yaml-%x", hash[:12])

		docs = append(docs, rag.Document{
			ID:      id,
			Content: content,
			Metadata: map[string]string{
				"source":          relPath,
				"type":            "yaml",
				"k8s_kind":        kind,
				"k8s_api_version": apiVersion,
				"k8s_name":        name,
			},
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking yaml dir %s: %w", dir, err)
	}
	return docs, nil
}

// extractMatch returns the first capturing group of re in text, or "".
func extractMatch(re *regexp.Regexp, text string) string {
	m := re.FindStringSubmatch(text)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}
