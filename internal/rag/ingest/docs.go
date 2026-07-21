package ingest

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/midu16/opm-troubleshooting/internal/rag"
)

var adocHeadingRe = regexp.MustCompile(`^(={1,4})\s+(.+)$`)
var adocIncludeRe = regexp.MustCompile(`^include::`)
var adocCommentRe = regexp.MustCompile(`^////`)
var adocConditionalRe = regexp.MustCompile(`^(ifdef|ifndef|endif|ifeval)::`)

// docTopics maps OCP doc section names to their directory paths inside
// the openshift-docs repository. These match the topics that the old
// html-single scraper fetched from docs.redhat.com.
var docTopics = []struct {
	Section string
	Dirs    []string
}{
	{"installing", []string{"installing"}},
	{"post_installation_configuration", []string{"post_installation_configuration"}},
	{"networking", []string{"networking"}},
	{"storage", []string{"storage"}},
	{"security", []string{"security"}},
	{"authentication", []string{"authentication"}},
	{"cicd", []string{"cicd"}},
	{"builds", []string{"builds"}},
	{"operators", []string{"operators"}},
	{"monitoring", []string{"monitoring"}},
	{"observability", []string{"observability"}},
	{"support", []string{"support"}},
	{"backup_and_restore", []string{"backup_and_restore"}},
	{"updating", []string{"updating"}},
	{"nodes", []string{"nodes"}},
	{"registry", []string{"registry"}},
	{"machine_management", []string{"machine_management"}},
	{"rest_api", []string{"rest_api"}},
	{"architecture", []string{"architecture"}},
	{"web_console", []string{"web_console"}},
	{"cli_reference", []string{"cli_reference"}},
	{"images", []string{"images"}},
	{"applications", []string{"applications"}},
	{"scalability_and_performance", []string{"scalability_and_performance"}},
	{"virt", []string{"virt"}},
	{"windows_containers", []string{"windows_containers"}},
	{"service_mesh", []string{"service_mesh"}},
	{"serverless", []string{"serverless"}},
	{"edge_computing", []string{"edge_computing"}},
}

// ScrapeOCPDocs clones or updates the openshift/openshift-docs GitHub
// repository and processes AsciiDoc files from the topic directories.
// This replaces the previous HTML scraper that fetched from docs.redhat.com,
// which now returns 403 for automated requests.
func ScrapeOCPDocs(ctx context.Context, cfg *rag.Config) ([]rag.Document, error) {
	docsRepoDir := filepath.Join(cfg.DocsDir(), "openshift-docs")

	if err := cloneOrUpdateDocsRepo(ctx, docsRepoDir, cfg.Ingestion.GitTimeout.Duration); err != nil {
		return nil, fmt.Errorf("openshift-docs repo: %w", err)
	}

	var allDocs []rag.Document

	for i, topic := range docTopics {
		select {
		case <-ctx.Done():
			return allDocs, ctx.Err()
		default:
		}

		fmt.Fprintf(os.Stderr, "  docs [%d/%d] %s ...\n", i+1, len(docTopics), topic.Section)

		for _, dir := range topic.Dirs {
			topicDir := filepath.Join(docsRepoDir, dir)
			if _, err := os.Stat(topicDir); err != nil {
				continue
			}

			chunks, err := processAdocDir(topicDir, topic.Section, cfg.OpenShift.Version)
			if err != nil {
				fmt.Fprintf(os.Stderr, "    warning: %s: %v\n", dir, err)
				continue
			}
			allDocs = append(allDocs, chunks...)
		}
	}

	return allDocs, nil
}

func cloneOrUpdateDocsRepo(ctx context.Context, repoDir string, timeout interface{ Seconds() float64 }) error {
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil {
		fmt.Fprintf(os.Stderr, "  openshift-docs: updating ...\n")
		cmd := exec.CommandContext(ctx, "git", "pull", "--ff-only")
		cmd.Dir = repoDir
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: git pull openshift-docs: %v\n", err)
		}
		return nil
	}

	fmt.Fprintf(os.Stderr, "  openshift-docs: cloning (depth=1, this may take a minute) ...\n")

	cmd := exec.CommandContext(ctx,
		"git", "clone",
		"--depth=1",
		"--filter=blob:none",
		"--no-checkout",
		"https://github.com/openshift/openshift-docs",
		repoDir,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}

	// Sparse checkout only the topic directories we need.
	sparseCmd := exec.CommandContext(ctx, "git", "sparse-checkout", "init", "--cone")
	sparseCmd.Dir = repoDir
	if err := sparseCmd.Run(); err != nil {
		return fmt.Errorf("sparse-checkout init: %w", err)
	}

	var dirs []string
	for _, t := range docTopics {
		dirs = append(dirs, t.Dirs...)
	}
	args := append([]string{"sparse-checkout", "set"}, dirs...)
	setCmd := exec.CommandContext(ctx, "git", args...)
	setCmd.Dir = repoDir
	if err := setCmd.Run(); err != nil {
		return fmt.Errorf("sparse-checkout set: %w", err)
	}

	checkoutCmd := exec.CommandContext(ctx, "git", "checkout")
	checkoutCmd.Dir = repoDir
	checkoutCmd.Stderr = os.Stderr
	if err := checkoutCmd.Run(); err != nil {
		return fmt.Errorf("git checkout: %w", err)
	}

	return nil
}

func processAdocDir(dir, section, ocpVersion string) ([]rag.Document, error) {
	var docs []rag.Document

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".adoc") {
			return nil
		}
		// Skip snippet/module files that are just include targets.
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

			hash := sha256.Sum256([]byte(section + ":" + relPath + ":" + chunk.heading))
			id := fmt.Sprintf("doc-%s-%x", section, hash[:12])

			docs = append(docs, rag.Document{
				ID:      id,
				Content: content,
				Metadata: map[string]string{
					"source":      "openshift-docs/" + section + "/" + relPath,
					"type":        "docs",
					"section":     section,
					"breadcrumb":  section + " > " + chunk.heading,
					"ocp_version": ocpVersion,
				},
			})
		}
		return nil
	})

	return docs, err
}

type adocChunk struct {
	heading string
	text    string
}

func splitAdocByHeading(text, section, relPath string) []adocChunk {
	lines := strings.Split(text, "\n")

	var chunks []adocChunk
	current := adocChunk{heading: strings.TrimSuffix(relPath, ".adoc")}
	var currentLines []string
	inComment := false

	for _, line := range lines {
		// Skip comment blocks.
		if adocCommentRe.MatchString(line) {
			inComment = !inComment
			continue
		}
		if inComment {
			continue
		}

		// Skip include directives and conditionals.
		if adocIncludeRe.MatchString(line) || adocConditionalRe.MatchString(line) {
			continue
		}

		// Check for AsciiDoc heading (= through ====).
		if m := adocHeadingRe.FindStringSubmatch(line); m != nil {
			if len(currentLines) > 0 {
				current.text = strings.Join(currentLines, "\n")
				chunks = append(chunks, current)
			}
			current = adocChunk{heading: strings.TrimSpace(m[2])}
			currentLines = nil
		} else {
			// Strip AsciiDoc formatting noise.
			cleaned := cleanAdocLine(line)
			if cleaned != "" {
				currentLines = append(currentLines, cleaned)
			}
		}
	}

	if len(currentLines) > 0 {
		current.text = strings.Join(currentLines, "\n")
		chunks = append(chunks, current)
	}

	return chunks
}

var adocAttrRe = regexp.MustCompile(`\{[a-zA-Z0-9_-]+\}`)
var adocInlineMacroRe = regexp.MustCompile(`(xref|link|image|btn|menu|kbd)::[^\[]*\[([^\]]*)\]`)
var adocRolePrefixRe = regexp.MustCompile(`^\[.*\]\s*$`)

func cleanAdocLine(line string) string {
	// Skip attribute definitions and role annotations.
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, ":") && strings.Contains(trimmed, ":") && len(trimmed) > 2 {
		parts := strings.SplitN(trimmed[1:], ":", 2)
		if len(parts) == 2 && !strings.Contains(parts[0], " ") {
			return ""
		}
	}
	if adocRolePrefixRe.MatchString(trimmed) {
		return ""
	}

	// Replace inline macros with their display text.
	line = adocInlineMacroRe.ReplaceAllString(line, "$2")

	// Strip bold/italic markers.
	line = strings.ReplaceAll(line, "**", "")
	line = strings.ReplaceAll(line, "__", "")
	line = strings.ReplaceAll(line, "``", "")

	// Remove single backtick code markers.
	line = strings.ReplaceAll(line, "`", "")

	return line
}
