package codeanalysis

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Match represents a source code location correlating with a failure symptom.
type Match struct {
	FilePath    string `json:"file_path"`
	LineNumber  int    `json:"line_number"`
	LineContent string `json:"line_content"`
	Pattern     string `json:"pattern"`
}

// Result holds code-level correlation findings.
type Result struct {
	RepoPath       string   `json:"repo_path"`
	SearchPatterns []string `json:"search_patterns"`
	Matches        []Match  `json:"matches"`
	Summary        string   `json:"summary"`
}

// Config specifies code analysis parameters.
type Config struct {
	RepoPath          string   // Local clone path
	SearchPatterns    []string // Error strings / log patterns to find
	MaxMatches        int
	IncludeExtensions []string
}

// Analyze searches operator source code for failure symptom patterns.
func Analyze(ctx context.Context, cfg Config) (*Result, error) {
	if cfg.RepoPath == "" {
		return nil, fmt.Errorf("repo path is required for code analysis")
	}
	if _, err := os.Stat(cfg.RepoPath); err != nil {
		return nil, fmt.Errorf("repo path not accessible: %w", err)
	}

	if cfg.MaxMatches <= 0 {
		cfg.MaxMatches = 20
	}
	if len(cfg.IncludeExtensions) == 0 {
		cfg.IncludeExtensions = []string{".go", ".py", ".yaml", ".yml", ".sh", ".ansible.yaml"}
	}

	result := &Result{
		RepoPath:       cfg.RepoPath,
		SearchPatterns: cfg.SearchPatterns,
		Matches:        make([]Match, 0),
	}

	for _, pattern := range cfg.SearchPatterns {
		if pattern == "" {
			continue
		}
		matches, err := searchRepo(ctx, cfg.RepoPath, pattern, cfg.IncludeExtensions, cfg.MaxMatches-len(result.Matches))
		if err != nil {
			continue
		}
		result.Matches = append(result.Matches, matches...)
		if len(result.Matches) >= cfg.MaxMatches {
			break
		}
	}

	result.Summary = buildSummary(result)
	return result, nil
}

// CloneAndAnalyze shallow-clones a repo and runs code correlation.
func CloneAndAnalyze(ctx context.Context, repoURL, commit string, patterns []string) (*Result, error) {
	tmpDir, err := os.MkdirTemp("", "codeanalysis-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", repoURL, tmpDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git clone: %w\n%s", err, output)
	}

	if commit != "" {
		fetch := exec.CommandContext(ctx, "git", "-C", tmpDir, "fetch", "origin", commit, "--depth=1")
		_ = fetch.Run()
		checkout := exec.CommandContext(ctx, "git", "-C", tmpDir, "checkout", commit)
		_ = checkout.Run()
	}

	return Analyze(ctx, Config{
		RepoPath:       tmpDir,
		SearchPatterns: patterns,
	})
}

func searchRepo(ctx context.Context, repoPath, pattern string, extensions []string, maxMatches int) ([]Match, error) {
	// Prefer git grep when available (faster, respects .gitignore)
	if matches, err := gitGrep(ctx, repoPath, pattern, maxMatches); err == nil && len(matches) > 0 {
		return matches, nil
	}

	return walkGrep(repoPath, pattern, extensions, maxMatches)
}

func gitGrep(ctx context.Context, repoPath, pattern string, maxMatches int) ([]Match, error) {
	args := []string{"-C", repoPath, "grep", "-n", "-i", "--no-color", pattern}
	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	return parseGrepOutput(string(output), pattern, maxMatches), nil
}

func walkGrep(repoPath, pattern string, extensions []string, maxMatches int) ([]Match, error) {
	matches := make([]Match, 0)
	patternLower := strings.ToLower(pattern)

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() && (info.Name() == ".git" || info.Name() == "vendor" || info.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}

		if !hasExtension(path, extensions) {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), patternLower) {
				rel, _ := filepath.Rel(repoPath, path)
				matches = append(matches, Match{
					FilePath:    rel,
					LineNumber:  i + 1,
					LineContent: strings.TrimSpace(line),
					Pattern:     pattern,
				})
				if len(matches) >= maxMatches {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})

	return matches, err
}

func parseGrepOutput(output, pattern string, maxMatches int) []Match {
	matches := make([]Match, 0)
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		// format: path:lineno:content
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		lineNum := 0
		_, _ = fmt.Sscanf(parts[1], "%d", &lineNum)
		matches = append(matches, Match{
			FilePath:    parts[0],
			LineNumber:  lineNum,
			LineContent: strings.TrimSpace(parts[2]),
			Pattern:     pattern,
		})
		if len(matches) >= maxMatches {
			break
		}
	}
	return matches
}

func hasExtension(path string, extensions []string) bool {
	for _, ext := range extensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

func buildSummary(result *Result) string {
	if len(result.Matches) == 0 {
		return "No source code matches found for failure patterns"
	}
	files := make(map[string]int)
	for _, m := range result.Matches {
		files[m.FilePath]++
	}
	return fmt.Sprintf("Found %d code correlation(s) across %d file(s)", len(result.Matches), len(files))
}

// PatternsFromFailure extracts searchable patterns from failure text and telco log patterns.
func PatternsFromFailure(failureReason string, extraPatterns []string) []string {
	seen := make(map[string]bool)
	patterns := make([]string, 0)

	add := func(p string) {
		p = strings.TrimSpace(p)
		if p != "" && len(p) >= 8 && !seen[p] {
			seen[p] = true
			patterns = append(patterns, p)
		}
	}

	for _, p := range extraPatterns {
		add(p)
	}

	// Extract quoted strings and error-like tokens
	for _, part := range strings.FieldsFunc(failureReason, func(r rune) bool {
		return r == ';' || r == ',' || r == '\n'
	}) {
		part = strings.TrimSpace(part)
		if strings.Contains(strings.ToLower(part), "error") ||
			strings.Contains(strings.ToLower(part), "failed") ||
			strings.Contains(strings.ToLower(part), "missing") {
			add(part)
		}
	}

	return patterns
}
