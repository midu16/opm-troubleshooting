package openshift

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

// GitHubIssue represents a GitHub issue search result.
type GitHubIssue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	Labels    []string  `json:"labels"`
	URL       string    `json:"html_url"`
	Body      string    `json:"body,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SearchIssues searches GitHub issues in the given repo using gh CLI or REST API.
func SearchIssues(ctx context.Context, repoPath, query string) ([]GitHubIssue, error) {
	if issues, err := searchWithGH(ctx, repoPath, query); err == nil {
		return issues, nil
	}
	return searchWithREST(ctx, repoPath, query)
}

func searchWithGH(ctx context.Context, repoPath, query string) ([]GitHubIssue, error) {
	cmd := exec.CommandContext(ctx, "gh", "search", "issues",
		"--repo", repoPath,
		"--json", "number,title,state,labels,url,body,createdAt,updatedAt",
		"--limit", "10",
		query)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh search: %w", err)
	}

	var ghResults []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		State  string `json:"state"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		URL       string    `json:"url"`
		Body      string    `json:"body"`
		CreatedAt time.Time `json:"createdAt"`
		UpdatedAt time.Time `json:"updatedAt"`
	}
	if err := json.Unmarshal(output, &ghResults); err != nil {
		return nil, fmt.Errorf("parse gh output: %w", err)
	}

	issues := make([]GitHubIssue, 0, len(ghResults))
	for _, r := range ghResults {
		labels := make([]string, 0, len(r.Labels))
		for _, l := range r.Labels {
			labels = append(labels, l.Name)
		}
		body := r.Body
		if len(body) > 500 {
			body = body[:500]
		}
		issues = append(issues, GitHubIssue{
			Number:    r.Number,
			Title:     r.Title,
			State:     r.State,
			Labels:    labels,
			URL:       r.URL,
			Body:      body,
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
		})
	}
	return issues, nil
}

func searchWithREST(ctx context.Context, repoPath, query string) ([]GitHubIssue, error) {
	searchQuery := fmt.Sprintf("%s repo:%s is:issue", query, repoPath)
	apiURL := fmt.Sprintf("https://api.github.com/search/issues?q=%s&per_page=10&sort=updated&order=desc",
		url.QueryEscape(searchQuery))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "opm-troubleshooting/1.0")

	if token := os.Getenv("GH_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	} else if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API returned %d", resp.StatusCode)
	}

	var result struct {
		Items []struct {
			Number    int       `json:"number"`
			Title     string    `json:"title"`
			State     string    `json:"state"`
			HTMLURL   string    `json:"html_url"`
			Body      string    `json:"body"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
			Labels    []struct {
				Name string `json:"name"`
			} `json:"labels"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode github response: %w", err)
	}

	issues := make([]GitHubIssue, 0, len(result.Items))
	for _, item := range result.Items {
		labels := make([]string, 0, len(item.Labels))
		for _, l := range item.Labels {
			labels = append(labels, l.Name)
		}
		body := item.Body
		if len(body) > 500 {
			body = body[:500]
		}
		issues = append(issues, GitHubIssue{
			Number:    item.Number,
			Title:     item.Title,
			State:     item.State,
			Labels:    labels,
			URL:       item.HTMLURL,
			Body:      body,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		})
	}
	return issues, nil
}

// ExtractSearchTerms builds search keywords from a failure reason string.
func ExtractSearchTerms(failureReason string) string {
	words := strings.Fields(failureReason)
	if len(words) > 8 {
		words = words[:8]
	}
	return strings.Join(words, " ")
}
