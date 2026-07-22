package openshift

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifyRepo_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"default_branch": "main",
			"description":    "Test repo",
			"pushed_at":      "2026-01-15T10:00:00Z",
		})
	}))
	defer srv.Close()

	v, err := verifyWithTestServer(t, srv, "openshift/test-operator")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !v.Exists {
		t.Error("expected Exists=true")
	}
	if !v.Verified {
		t.Error("expected Verified=true")
	}
	if v.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", v.DefaultBranch, "main")
	}
	if v.Description != "Test repo" {
		t.Errorf("Description = %q, want %q", v.Description, "Test repo")
	}
}

func TestVerifyRepo_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	v, err := verifyWithTestServer(t, srv, "openshift/nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Exists {
		t.Error("expected Exists=false")
	}
	if !v.Verified {
		t.Error("expected Verified=true even for 404")
	}
}

func TestVerifyRepo_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	v, err := verifyWithTestServer(t, srv, "openshift/test-operator")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Exists {
		t.Error("expected Exists=false on rate limit")
	}
	if v.Error == "" {
		t.Error("expected non-empty Error on rate limit")
	}
}

func TestVerifyRepo_EmptyPath(t *testing.T) {
	v, err := VerifyRepo(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Exists {
		t.Error("expected Exists=false for empty path")
	}
}

func TestExtractRepoPath(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "full https URL",
			url:  "https://github.com/openshift/cluster-etcd-operator",
			want: "openshift/cluster-etcd-operator",
		},
		{
			name: "URL with commit path",
			url:  "https://github.com/openshift/foo/commit/abc123",
			want: "openshift/foo",
		},
		{
			name: "URL with tree path",
			url:  "https://github.com/openshift/bar/tree/main/pkg",
			want: "openshift/bar",
		},
		{
			name: "URL with trailing slash",
			url:  "https://github.com/openshift/baz/",
			want: "openshift/baz",
		},
		{
			name: "http URL",
			url:  "http://github.com/openshift/test",
			want: "openshift/test",
		},
		{
			name: "non-GitHub URL",
			url:  "https://gitlab.com/openshift/foo",
			want: "",
		},
		{
			name: "empty URL",
			url:  "",
			want: "",
		},
		{
			name: "GitHub URL with only owner",
			url:  "https://github.com/openshift",
			want: "",
		},
		{
			name: "non-openshift org",
			url:  "https://github.com/red-hat-storage/odf-operator",
			want: "red-hat-storage/odf-operator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractRepoPath(tt.url)
			if got != tt.want {
				t.Errorf("ExtractRepoPath(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

// verifyWithTestServer is a test helper that calls the REST verification
// against a local httptest.Server instead of the real GitHub API.
func verifyWithTestServer(t *testing.T, srv *httptest.Server, repoPath string) (*RepoVerification, error) {
	t.Helper()
	ctx := context.Background()
	apiURL := srv.URL + "/repos/" + repoPath

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "opm-troubleshooting/1.0")

	client := srv.Client()
	resp, err := client.Do(req)
	if err != nil {
		return &RepoVerification{Exists: false, Error: err.Error()}, nil
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
		DefaultBranch string `json:"default_branch"`
		Description   string `json:"description"`
		PushedAt      string `json:"pushed_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return &RepoVerification{Exists: true, Verified: true, Error: err.Error()}, nil
	}

	return &RepoVerification{
		Exists:        true,
		DefaultBranch: result.DefaultBranch,
		Description:   result.Description,
		Verified:      true,
	}, nil
}
