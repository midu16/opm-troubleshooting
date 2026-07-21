package ingest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/midu16/opm-troubleshooting/internal/rag"
)

const (
	telcoRepoURL    = "https://github.com/openshift-kni/telco-reference.git"
	telcoRepoBranch = "release-4.22"
)

// LoadTelcoReference clones (or updates) the openshift-kni/telco-reference
// repository at the release-4.22 branch and loads all YAML and Markdown
// files from the telco-core/ and telco-ran/ subdirectories.
func LoadTelcoReference(ctx context.Context, cfg *rag.Config) ([]rag.Document, error) {
	telcoDir := cfg.TelcoDir()

	if err := cloneOrUpdateTelco(ctx, cfg, telcoDir); err != nil {
		return nil, fmt.Errorf("clone/update telco-reference: %w", err)
	}

	var allDocs []rag.Document

	profiles := []struct {
		subDir  string
		profile string
	}{
		{subDir: "telco-core", profile: "telco-core"},
		{subDir: "telco-ran", profile: "telco-ran"},
	}

	for _, p := range profiles {
		dir := filepath.Join(telcoDir, p.subDir)
		if _, err := os.Stat(dir); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: telco subdir %s not found\n", p.subDir)
			continue
		}

		// Load YAML files.
		yamlDocs, err := LoadYAMLDir(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warning: loading yaml from %s: %v\n", p.subDir, err)
		} else {
			for i := range yamlDocs {
				yamlDocs[i].Metadata["telco_profile"] = p.profile
			}
			allDocs = append(allDocs, yamlDocs...)
		}

		// Load Markdown files.
		mdDocs, err := ChunkMarkdown(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warning: loading markdown from %s: %v\n", p.subDir, err)
		} else {
			for i := range mdDocs {
				mdDocs[i].Metadata["telco_profile"] = p.profile
			}
			allDocs = append(allDocs, mdDocs...)
		}
	}

	return allDocs, nil
}

// cloneOrUpdateTelco clones the telco-reference repo if absent, or does
// a fast-forward pull if already present.
func cloneOrUpdateTelco(ctx context.Context, cfg *rag.Config, telcoDir string) error {
	timeout := cfg.Ingestion.GitTimeout.Duration

	if _, err := os.Stat(filepath.Join(telcoDir, ".git")); err == nil {
		// Already cloned — pull.
		fmt.Fprintf(os.Stderr, "  telco-reference: updating ...\n")
		cmdCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		cmd := exec.CommandContext(cmdCtx, "git", "pull", "--ff-only")
		cmd.Dir = telcoDir
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: git pull telco-reference: %v\n", err)
		}
		return nil
	}

	// Clone fresh.
	fmt.Fprintf(os.Stderr, "  telco-reference: cloning %s ...\n", telcoRepoBranch)
	if err := os.MkdirAll(filepath.Dir(telcoDir), 0755); err != nil {
		return err
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "git", "clone", "--depth=1", "--branch", telcoRepoBranch, telcoRepoURL, telcoDir)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
