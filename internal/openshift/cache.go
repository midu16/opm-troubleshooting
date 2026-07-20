package openshift

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CloneOrUpdate ensures a shallow clone exists in cacheDir and returns the local path.
func CloneOrUpdate(ctx context.Context, repoPath, cacheDir string) (string, error) {
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		cacheDir = filepath.Join(home, ".config", "opm-troubleshooting", "repos")
	}
	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}

	safeName := strings.ReplaceAll(repoPath, "/", "_")
	localPath := filepath.Join(cacheDir, safeName)

	if isGitRepo(localPath) {
		if shouldUpdate(localPath) {
			_ = gitFetchShallow(ctx, localPath)
		}
		return localPath, nil
	}

	repoURL := "https://github.com/" + repoPath + ".git"
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", "--single-branch", repoURL, localPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git clone %s: %w\n%s", repoPath, err, output)
	}
	return localPath, nil
}

// RecentCommits returns commits from the last N days matching optional keywords.
func RecentCommits(ctx context.Context, repoDir string, days int, keywords []string) ([]CommitInfo, error) {
	since := fmt.Sprintf("--since=%dd", days)
	args := []string{"-C", repoDir, "log", "--oneline", since, "--format=%H|%s|%an|%aI"}
	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	var commits []CommitInfo
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		ci := CommitInfo{
			Hash:    parts[0],
			Subject: parts[1],
			Author:  parts[2],
			Date:    parts[3],
		}

		if len(keywords) == 0 {
			commits = append(commits, ci)
			continue
		}
		subjectLower := strings.ToLower(ci.Subject)
		for _, kw := range keywords {
			if strings.Contains(subjectLower, strings.ToLower(kw)) {
				commits = append(commits, ci)
				break
			}
		}
	}
	return commits, nil
}

// CommitInfo holds metadata for a git commit.
type CommitInfo struct {
	Hash    string `json:"hash"`
	Subject string `json:"subject"`
	Author  string `json:"author"`
	Date    string `json:"date"`
}

func isGitRepo(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil && info.IsDir()
}

func shouldUpdate(repoDir string) bool {
	info, err := os.Stat(filepath.Join(repoDir, ".git", "FETCH_HEAD"))
	if err != nil {
		return true
	}
	return time.Since(info.ModTime()) > 24*time.Hour
}

func gitFetchShallow(ctx context.Context, repoDir string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "fetch", "--depth=1")
	_, err := cmd.CombinedOutput()
	return err
}
