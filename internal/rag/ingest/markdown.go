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

var headingRe = regexp.MustCompile(`^(#{1,4})\s+(.+)$`)

// skipDirs lists directory names to skip during walks.
var skipDirs = map[string]bool{
	"vendor":       true,
	"node_modules": true,
	".git":         true,
}

// ChunkMarkdown walks dir for *.md files, splits them by heading
// boundaries (# through ####), and returns one Document per chunk with
// breadcrumb hierarchy metadata.
func ChunkMarkdown(dir string) ([]rag.Document, error) {
	var docs []rag.Document

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) != ".md" {
			return nil
		}

		relPath, _ := filepath.Rel(dir, path)
		chunks, chunkErr := chunkMarkdownFile(path, relPath)
		if chunkErr != nil {
			return nil // skip individual file errors
		}
		docs = append(docs, chunks...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking markdown dir %s: %w", dir, err)
	}
	return docs, nil
}

// chunkMarkdownFile splits a single markdown file into heading-bounded chunks.
func chunkMarkdownFile(path, relPath string) ([]rag.Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")

	type section struct {
		level   int
		heading string
		lines   []string
	}

	var sections []section
	current := section{level: 0, heading: ""}

	for _, line := range lines {
		m := headingRe.FindStringSubmatch(line)
		if m != nil {
			// Flush current section
			if len(current.lines) > 0 || current.heading != "" {
				sections = append(sections, current)
			}
			level := len(m[1])
			current = section{
				level:   level,
				heading: strings.TrimSpace(m[2]),
				lines:   nil,
			}
		} else {
			current.lines = append(current.lines, line)
		}
	}
	// Flush last section.
	if len(current.lines) > 0 || current.heading != "" {
		sections = append(sections, current)
	}

	// Build documents with breadcrumb hierarchy.
	docs := make([]rag.Document, 0, len(sections))
	// Track headings per level for breadcrumb.
	headings := make(map[int]string) // level -> heading text

	for _, sec := range sections {
		content := strings.TrimSpace(strings.Join(sec.lines, "\n"))
		if content == "" && sec.heading == "" {
			continue
		}

		// Update heading hierarchy.
		if sec.level > 0 {
			headings[sec.level] = sec.heading
			// Clear deeper levels.
			for l := sec.level + 1; l <= 6; l++ {
				delete(headings, l)
			}
		}

		// Build breadcrumb from the heading hierarchy.
		breadcrumb := buildBreadcrumb(headings, sec.level)

		// Prepend heading to content for context.
		if sec.heading != "" {
			content = sec.heading + "\n\n" + content
		}

		if content == "" {
			continue
		}

		hash := sha256.Sum256([]byte(relPath + ":" + sec.heading + ":" + content))
		id := fmt.Sprintf("md-%x", hash[:12])

		docs = append(docs, rag.Document{
			ID:      id,
			Content: content,
			Metadata: map[string]string{
				"source":     relPath,
				"type":       "markdown",
				"section":    sec.heading,
				"breadcrumb": breadcrumb,
			},
		})
	}
	return docs, nil
}

// buildBreadcrumb constructs a breadcrumb string like "Installation > Prerequisites > Hardware".
func buildBreadcrumb(headings map[int]string, currentLevel int) string {
	var parts []string
	for l := 1; l <= currentLevel; l++ {
		if h, ok := headings[l]; ok && h != "" {
			parts = append(parts, h)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " > ")
}
