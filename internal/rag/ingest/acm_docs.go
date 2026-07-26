package ingest

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/midu16/opm-troubleshooting/internal/rag"
)

// acmDocTopics lists the topic directories inside the stolostron/rhacm-docs
// repository that contain ACM and MCE documentation.
var acmDocTopics = []struct {
	Section string
	Dir     string
}{
	{"about", "about"},
	{"add-ons", "add-ons"},
	{"apis", "apis"},
	{"applications", "applications"},
	{"business_continuity", "business_continuity"},
	{"clusters", "clusters"},
	{"console", "console"},
	{"gitops", "gitops"},
	{"global_hub", "global_hub"},
	{"governance", "governance"},
	{"health_metrics", "health_metrics"},
	{"install", "install"},
	{"mce_acm_integration", "mce_acm_integration"},
	{"networking", "networking"},
	{"observability", "observability"},
	{"release_notes", "release_notes"},
	{"search", "search"},
	{"secure_clusters", "secure_clusters"},
	{"troubleshooting", "troubleshooting"},
	{"virtualization", "virtualization"},
}

// ScrapeACMDocs clones or updates the stolostron/rhacm-docs GitHub
// repository and processes AsciiDoc files from the topic directories.
func ScrapeACMDocs(ctx context.Context, cfg *rag.Config, branch string) ([]rag.Document, error) {
	acmRepoDir := cfg.ACMDocsDir()

	repoURL := "https://github.com/" + cfg.OpenShift.ACMDocs.Repo
	if err := cloneOrUpdateACMDocsRepo(ctx, acmRepoDir, repoURL, branch); err != nil {
		return nil, fmt.Errorf("rhacm-docs repo: %w", err)
	}

	var allDocs []rag.Document

	for i, topic := range acmDocTopics {
		select {
		case <-ctx.Done():
			return allDocs, ctx.Err()
		default:
		}

		fmt.Fprintf(os.Stderr, "  acm-docs [%d/%d] %s ...\n", i+1, len(acmDocTopics), topic.Section)

		topicDir := filepath.Join(acmRepoDir, topic.Dir)
		if _, err := os.Stat(topicDir); err != nil {
			continue
		}

		chunks, err := processACMAdocDir(topicDir, topic.Section)
		if err != nil {
			fmt.Fprintf(os.Stderr, "    warning: %s: %v\n", topic.Dir, err)
			continue
		}
		allDocs = append(allDocs, chunks...)
	}

	return allDocs, nil
}

func cloneOrUpdateACMDocsRepo(ctx context.Context, repoDir, repoURL, branch string) error {
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil {
		fmt.Fprintf(os.Stderr, "  rhacm-docs: updating ...\n")
		cmd := exec.CommandContext(ctx, "git", "pull", "--ff-only")
		cmd.Dir = repoDir
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: git pull rhacm-docs: %v\n", err)
		}
		return nil
	}

	fmt.Fprintf(os.Stderr, "  rhacm-docs: cloning branch %s (depth=1) ...\n", branch)

	cloneArgs := []string{"clone", "--depth=1"}
	if branch != "" {
		cloneArgs = append(cloneArgs, "--branch", branch)
	}
	cloneArgs = append(cloneArgs, repoURL, repoDir)
	cmd := exec.CommandContext(ctx, "git", cloneArgs...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}

	return nil
}

func processACMAdocDir(dir, section string) ([]rag.Document, error) {
	var docs []rag.Document

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".adoc") {
			return nil
		}
		base := filepath.Base(path)
		if strings.HasPrefix(base, "_") || strings.HasPrefix(base, "snippets-") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(dir, path)
		chunks := splitAdocByHeading(string(data), section, relPath)

		for _, chunk := range chunks {
			content := strings.TrimSpace(chunk.text)
			if len(content) < 30 {
				continue
			}

			hash := sha256.Sum256([]byte("acm:" + section + ":" + relPath + ":" + chunk.heading))
			id := fmt.Sprintf("acm-%s-%x", section, hash[:12])

			docs = append(docs, rag.Document{
				ID:      id,
				Content: content,
				Metadata: map[string]string{
					"source":     "rhacm-docs/" + section + "/" + relPath,
					"type":       "acm_docs",
					"section":    section,
					"breadcrumb": "ACM > " + section + " > " + chunk.heading,
					"product":    "acm",
				},
			})
		}
		return nil
	})

	return docs, err
}
