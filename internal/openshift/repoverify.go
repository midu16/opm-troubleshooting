package openshift

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// RepoVerification holds the result of verifying a GitHub repo exists.
type RepoVerification struct {
	Exists        bool      `json:"exists"`
	DefaultBranch string    `json:"default_branch,omitempty"`
	Description   string    `json:"description,omitempty"`
	LastPushedAt  time.Time `json:"last_pushed_at,omitempty"`
	Verified      bool      `json:"verified"`
	Error         string    `json:"error,omitempty"`
}

// VerifyRepo checks whether a GitHub repository exists by hitting the GitHub API.
// repoPath should be in "owner/repo" format (e.g. "openshift/cluster-etcd-operator").
// Returns Exists=true on 200, Exists=false on 404, and a non-fatal error on network/rate-limit failures.
func VerifyRepo(ctx context.Context, repoPath string) (*RepoVerification, error) {
	if repoPath == "" {
		return &RepoVerification{Exists: false, Error: "empty repo path"}, nil
	}

	if v, err := verifyWithGH(ctx, repoPath); err == nil {
		return v, nil
	}
	return verifyWithREST(ctx, repoPath)
}

func verifyWithGH(ctx context.Context, repoPath string) (*RepoVerification, error) {
	cmd := exec.CommandContext(ctx, "gh", "repo", "view", repoPath,
		"--json", "name,defaultBranchRef,description,pushedAt")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh repo view: %w", err)
	}

	var ghResult struct {
		Name             string `json:"name"`
		DefaultBranchRef struct {
			Name string `json:"name"`
		} `json:"defaultBranchRef"`
		Description string    `json:"description"`
		PushedAt    time.Time `json:"pushedAt"`
	}
	if err := json.Unmarshal(output, &ghResult); err != nil {
		return nil, fmt.Errorf("parse gh output: %w", err)
	}

	return &RepoVerification{
		Exists:        true,
		DefaultBranch: ghResult.DefaultBranchRef.Name,
		Description:   ghResult.Description,
		LastPushedAt:  ghResult.PushedAt,
		Verified:      true,
	}, nil
}

func verifyWithREST(ctx context.Context, repoPath string) (*RepoVerification, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s", repoPath)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, http.NoBody)
	if err != nil {
		return &RepoVerification{Exists: false, Error: err.Error()}, nil
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "opm-troubleshooting/1.0")

	if token := os.Getenv("GH_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	} else if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return &RepoVerification{Exists: false, Error: fmt.Sprintf("github API: %v", err)}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &RepoVerification{Exists: false, Verified: true}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return &RepoVerification{
			Exists: false,
			Error:  fmt.Sprintf("github API returned %d", resp.StatusCode),
		}, nil
	}

	var result struct {
		DefaultBranch string    `json:"default_branch"`
		Description   string    `json:"description"`
		PushedAt      time.Time `json:"pushed_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return &RepoVerification{
			Exists:   true,
			Verified: true,
			Error:    fmt.Sprintf("decode response: %v", err),
		}, nil
	}

	return &RepoVerification{
		Exists:        true,
		DefaultBranch: result.DefaultBranch,
		Description:   result.Description,
		LastPushedAt:  result.PushedAt,
		Verified:      true,
	}, nil
}

// ExtractRepoPath converts a full GitHub URL to an "owner/repo" path.
// For example: "https://github.com/openshift/foo/commit/abc" → "openshift/foo".
// Returns an empty string for non-GitHub URLs.
func ExtractRepoPath(fullURL string) string {
	if fullURL == "" {
		return ""
	}

	for _, prefix := range []string{
		"https://github.com/",
		"http://github.com/",
	} {
		if !strings.HasPrefix(fullURL, prefix) {
			continue
		}
		path := strings.TrimPrefix(fullURL, prefix)
		path = strings.TrimSuffix(path, "/")
		parts := strings.SplitN(path, "/", 3)
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			return parts[0] + "/" + parts[1]
		}
		return ""
	}

	return ""
}
