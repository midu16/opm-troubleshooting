package ingest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/midu16/opm-troubleshooting/internal/rag"
)

// telcoProfiles maps subdirectory names to their profile label.
var telcoProfiles = map[string]string{
	"telco-core": "telco-core",
	"telco-ran":  "telco-ran",
	"telco-hub":  "telco-hub",
}

// LoadTelcoReference clones (or updates) the telco-reference repository
// configured in cfg.OpenShift.TelcoReference.Repo at the specified branch
// and loads all YAML, Markdown, and Go source files from the entire
// repository tree. Documents under known profile subdirectories
// (telco-core, telco-ran, telco-hub) are tagged with the corresponding
// telco_profile metadata.
func LoadTelcoReference(ctx context.Context, cfg *rag.Config, telcoBranch string) ([]rag.Document, error) {
	telcoDir := cfg.TelcoDir()
	repoSlug := cfg.OpenShift.TelcoReference.Repo
	repoURL := cfg.RepoURL(repoSlug)

	if err := cloneOrUpdateTelco(ctx, cfg, telcoDir, repoURL, telcoBranch); err != nil {
		return nil, fmt.Errorf("clone/update %s: %w", repoSlug, err)
	}

	var allDocs []rag.Document

	// Load YAML files from the entire repo.
	yamlDocs, err := LoadYAMLDir(telcoDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  warning: loading yaml from %s: %v\n", repoSlug, err)
	} else {
		tagTelcoProfile(cfg, yamlDocs, repoSlug)
		allDocs = append(allDocs, yamlDocs...)
	}

	// Load Markdown files from the entire repo.
	mdDocs, err := ChunkMarkdown(telcoDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  warning: loading markdown from %s: %v\n", repoSlug, err)
	} else {
		tagTelcoProfile(cfg, mdDocs, repoSlug)
		allDocs = append(allDocs, mdDocs...)
	}

	// Load Go source files (consistent with other repos).
	ve := cfg.ActiveVersionEntry()
	goDocs, err := ChunkGoSource(telcoDir, repoSlug, ve.Version, cfg.GitBase(), &cfg.Secret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  warning: loading go source from %s: %v\n", repoSlug, err)
	} else {
		tagTelcoProfile(cfg, goDocs, repoSlug)
		allDocs = append(allDocs, goDocs...)
	}

	return allDocs, nil
}

// tagTelcoProfile sets the telco_profile metadata field on documents whose
// source path falls under a known profile subdirectory, and tags all
// documents with the repo origin.
func tagTelcoProfile(cfg *rag.Config, docs []rag.Document, repoSlug string) {
	repoURL := cfg.RepoURL(repoSlug)
	for i := range docs {
		src := docs[i].Metadata["source"]
		for dir, profile := range telcoProfiles {
			if strings.HasPrefix(src, dir+"/") || strings.HasPrefix(src, dir+string(filepath.Separator)) {
				docs[i].Metadata["telco_profile"] = profile
				break
			}
		}
		docs[i].Metadata["repo"] = repoSlug
		docs[i].Metadata["repo_url"] = repoURL
	}
}

// cloneOrUpdateTelco clones the telco-reference repo if absent, or does
// a fast-forward pull if already present.
func cloneOrUpdateTelco(ctx context.Context, cfg *rag.Config, telcoDir, repoURL, telcoBranch string) error {
	timeout := cfg.Ingestion.GitTimeout.Duration

	if _, err := os.Stat(filepath.Join(telcoDir, ".git")); err == nil {
		// Already cloned — pull.
		fmt.Fprintf(os.Stderr, "  %s: updating ...\n", cfg.OpenShift.TelcoReference.Repo)
		cmdCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		cmd := exec.CommandContext(cmdCtx, "git", "pull", "--ff-only")
		cmd.Dir = telcoDir
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: git pull %s: %v\n", cfg.OpenShift.TelcoReference.Repo, err)
		}
		return nil
	}

	// Clone fresh.
	fmt.Fprintf(os.Stderr, "  %s: cloning %s ...\n", cfg.OpenShift.TelcoReference.Repo, telcoBranch)
	if err := os.MkdirAll(filepath.Dir(telcoDir), 0o755); err != nil {
		return err
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "git", "clone", "--depth=1", "--branch", telcoBranch, repoURL, telcoDir)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
