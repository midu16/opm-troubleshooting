package gitdelta

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// CalculateDelta clones the repository (shallow) and computes the diff between two commits.
func CalculateDelta(ctx context.Context, req DeltaRequest) (*CommitDelta, error) {
	if req.OldCommit == req.NewCommit {
		return &CommitDelta{
			RepoURL:   req.RepoURL,
			OldCommit: req.OldCommit,
			NewCommit: req.NewCommit,
		}, nil
	}

	// Create temp directory for clone
	tmpDir, err := os.MkdirTemp("", "gitdelta-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Shallow clone with both commits
	if err := shallowClone(ctx, req.RepoURL, tmpDir, req.OldCommit, req.NewCommit); err != nil {
		return nil, fmt.Errorf("clone repo: %w", err)
	}

	// Run git diff
	diffOutput, err := runGitDiff(ctx, tmpDir, req.OldCommit, req.NewCommit)
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	// Parse diff stats
	stats, err := runGitDiffStats(ctx, tmpDir, req.OldCommit, req.NewCommit)
	if err != nil {
		return nil, fmt.Errorf("git diff stats: %w", err)
	}

	return &CommitDelta{
		RepoURL:      req.RepoURL,
		OldCommit:    req.OldCommit,
		NewCommit:    req.NewCommit,
		DiffSummary:  diffOutput,
		FilesChanged: parseFilesFromStats(stats),
		Additions:    parseAdditions(stats),
		Deletions:    parseDeletions(stats),
	}, nil
}

func shallowClone(ctx context.Context, repoURL, destDir, commit1, commit2 string) error {
	// git clone --depth=1 --no-single-branch <repo> <destDir>
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", "--no-single-branch", repoURL, destDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone failed: %w\noutput: %s", err, output)
	}

	// git fetch origin <commit1> <commit2> --depth=1
	cmd = exec.CommandContext(ctx, "git", "-C", destDir, "fetch", "origin", commit1, commit2, "--depth=1")
	if err := cmd.Run(); err != nil {
		// Commits might already be in shallow clone - this is non-fatal
		return nil
	}

	return nil
}

func runGitDiff(ctx context.Context, repoPath, oldCommit, newCommit string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "diff", oldCommit, newCommit)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w\noutput: %s", err, output)
	}
	return string(output), nil
}

func runGitDiffStats(ctx context.Context, repoPath, oldCommit, newCommit string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "diff", "--stat", oldCommit, newCommit)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func parseFilesFromStats(stats string) []string {
	lines := strings.Split(stats, "\n")
	files := make([]string, 0)
	for _, line := range lines {
		if strings.Contains(line, "|") {
			parts := strings.Split(line, "|")
			if len(parts) > 0 {
				files = append(files, strings.TrimSpace(parts[0]))
			}
		}
	}
	return files
}

func parseAdditions(stats string) int {
	// Parse from summary line: "X files changed, Y insertions(+), Z deletions(-)"
	if strings.Contains(stats, "insertion") {
		return extractNumber(stats, "insertion")
	}
	return 0
}

func parseDeletions(stats string) int {
	if strings.Contains(stats, "deletion") {
		return extractNumber(stats, "deletion")
	}
	return 0
}

func extractNumber(s, keyword string) int {
	idx := strings.Index(s, keyword)
	if idx == -1 {
		return 0
	}

	// Walk backwards to find the number
	start := idx - 1
	for start >= 0 && (s[start] == ' ' || s[start] == ',') {
		start--
	}

	end := start + 1
	for start >= 0 && s[start] >= '0' && s[start] <= '9' {
		start--
	}
	start++

	numStr := strings.TrimSpace(s[start:end])
	num, _ := strconv.Atoi(numStr)
	return num
}
