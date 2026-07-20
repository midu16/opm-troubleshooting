package openshift

import (
	"testing"
)

func TestLookupRepo(t *testing.T) {
	tests := []struct {
		name          string
		operator      string
		wantRepo      string
		wantComponent string
	}{
		{
			name:          "etcd cluster operator",
			operator:      "etcd",
			wantRepo:      "openshift/cluster-etcd-operator",
			wantComponent: "etcd",
		},
		{
			name:          "network cluster operator",
			operator:      "network",
			wantRepo:      "openshift/cluster-network-operator",
			wantComponent: "Network",
		},
		{
			name:          "sriov OLM operator",
			operator:      "sriov-network-operator",
			wantRepo:      "openshift/sriov-network-operator",
			wantComponent: "SR-IOV",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, ok := LookupRepo(tt.operator)
			if !ok {
				t.Fatalf("LookupRepo(%q) returned not-found, expected match", tt.operator)
			}
			if info.Repo != tt.wantRepo {
				t.Errorf("Repo = %q, want %q", info.Repo, tt.wantRepo)
			}
			if info.Component != tt.wantComponent {
				t.Errorf("Component = %q, want %q", info.Component, tt.wantComponent)
			}
		})
	}
}

func TestLookupRepoNotFound(t *testing.T) {
	_, ok := LookupRepo("nonexistent-operator-xyz")
	if ok {
		t.Error("LookupRepo for unknown operator should return false")
	}
}

func TestLookupByComponent(t *testing.T) {
	tests := []struct {
		name      string
		component string
		wantLen   int
		wantRepo  string // one expected repo in results
	}{
		{
			name:      "find SR-IOV component",
			component: "SR-IOV",
			wantLen:   1,
			wantRepo:  "openshift/sriov-network-operator",
		},
		{
			name:      "find etcd component",
			component: "etcd",
			wantLen:   1,
			wantRepo:  "openshift/cluster-etcd-operator",
		},
		{
			name:      "nonexistent component returns empty",
			component: "DoesNotExist",
			wantLen:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := LookupByComponent(tt.component)
			if len(results) != tt.wantLen {
				t.Fatalf("LookupByComponent(%q) returned %d results, want %d", tt.component, len(results), tt.wantLen)
			}
			if tt.wantLen > 0 {
				found := false
				for _, r := range results {
					if r.Repo == tt.wantRepo {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected repo %q in results, not found", tt.wantRepo)
				}
			}
		})
	}
}

func TestRepoURL(t *testing.T) {
	tests := []struct {
		repoPath string
		wantURL  string
	}{
		{
			repoPath: "openshift/cluster-etcd-operator",
			wantURL:  "https://github.com/openshift/cluster-etcd-operator",
		},
		{
			repoPath: "openshift-kni/numaresources-operator",
			wantURL:  "https://github.com/openshift-kni/numaresources-operator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.repoPath, func(t *testing.T) {
			got := RepoURL(tt.repoPath)
			if got != tt.wantURL {
				t.Errorf("RepoURL(%q) = %q, want %q", tt.repoPath, got, tt.wantURL)
			}
		})
	}
}

func TestAllRepos(t *testing.T) {
	repos := AllRepos()

	if len(repos) < 30 {
		t.Errorf("AllRepos() returned %d repos, want at least 30", len(repos))
	}

	// Verify uniqueness
	seen := make(map[string]bool)
	for _, r := range repos {
		if seen[r] {
			t.Errorf("AllRepos() returned duplicate repo: %q", r)
		}
		seen[r] = true
	}
}

func TestRegistryCoverage(t *testing.T) {
	for name, info := range RepoRegistry {
		if info.Repo == "" {
			t.Errorf("RepoRegistry[%q] has empty Repo field", name)
		}
		if info.Component == "" {
			t.Errorf("RepoRegistry[%q] has empty Component field", name)
		}
	}
}
