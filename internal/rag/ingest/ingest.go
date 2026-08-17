package ingest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/midu16/opm-troubleshooting/internal/rag"
)

// RunIngestion orchestrates the full RAG ingestion pipeline:
//  1. Creates data directories
//  2. Clones/updates all OpenShift repos (concurrent)
//  3. Loads telco-reference configs
//  4. Scrapes OCP docs (with caching)
//  5. Scrapes ACM/MCE docs (with caching)
//  6. Resets and populates each collection
//  7. Saves freshness metadata
func RunIngestion(ctx context.Context, cfg *rag.Config, store rag.VectorStore) error {
	reposDir := cfg.ReposDir()
	docsDir := cfg.DocsDir()

	fmt.Fprintf(os.Stderr, "=== OCP RAG Ingestion Pipeline ===\n")

	// Step 1: Create data directories.
	for _, dir := range []string{reposDir, docsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	// Resolve version entries for branch-aware operations.
	ve := cfg.ActiveVersionEntry()

	// Step 2: Clone/update all OpenShift repos concurrently.
	fmt.Fprintf(os.Stderr, "\n[1/7] Cloning/updating %d OpenShift repos (branch: %s) ...\n", len(cfg.OpenShift.Repos), ve.OperatorBranch)
	repoCommits, err := cloneAllRepos(ctx, cfg, reposDir, ve.OperatorBranch)
	if err != nil {
		return fmt.Errorf("clone repos: %w", err)
	}

	// Step 3: Load telco-reference content.
	var telcoDocs []rag.Document
	if cfg.OpenShift.TelcoReference.Enabled {
		fmt.Fprintf(os.Stderr, "\n[2/7] Loading %s (branch: %s) ...\n", cfg.OpenShift.TelcoReference.Repo, ve.TelcoBranch)
		telcoDocs, err = LoadTelcoReference(ctx, cfg, ve.TelcoBranch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warning: %s: %v\n", cfg.OpenShift.TelcoReference.Repo, err)
		}
		fmt.Fprintf(os.Stderr, "  loaded %d telco documents\n", len(telcoDocs))
	} else {
		fmt.Fprintf(os.Stderr, "\n[2/7] Telco-reference ingestion disabled, skipping.\n")
	}

	// Step 4: Scrape OCP docs.
	fmt.Fprintf(os.Stderr, "\n[3/7] Scraping OCP %s docs (branch: %s) ...\n", cfg.OpenShift.Version, ve.DocsBranch)
	ocpDocs, err := ScrapeOCPDocs(ctx, cfg, ve.DocsBranch)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  warning: OCP docs: %v\n", err)
	}
	fmt.Fprintf(os.Stderr, "  scraped %d doc chunks\n", len(ocpDocs))

	// Step 5: Scrape ACM/MCE docs.
	var acmDocs []rag.Document
	if cfg.OpenShift.ACMDocs.Enabled {
		acmBranch := ve.ACMDocsBranch
		if acmBranch == "" {
			acmBranch = "2.17_stage"
		}
		fmt.Fprintf(os.Stderr, "\n[4/7] Scraping ACM/MCE docs (branch: %s) ...\n", acmBranch)
		acmDocs, err = ScrapeACMDocs(ctx, cfg, acmBranch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warning: ACM docs: %v\n", err)
		}
		fmt.Fprintf(os.Stderr, "  scraped %d ACM doc chunks\n", len(acmDocs))
	} else {
		fmt.Fprintf(os.Stderr, "\n[4/7] ACM/MCE docs ingestion disabled, skipping.\n")
	}

	// Step 6: Build all collection documents and populate the store.
	fmt.Fprintf(os.Stderr, "\n[5/7] Building and indexing collections ...\n")

	// Build code docs from all repos.
	var codeDocs []rag.Document
	for _, repo := range cfg.OpenShift.Repos {
		repoDir := filepath.Join(reposDir, repo)
		if _, err := os.Stat(repoDir); err != nil {
			continue
		}
		fmt.Fprintf(os.Stderr, "  chunking Go source: %s ...\n", repo)
		repoDocs, err := ChunkGoSource(repoDir, repo, ve.Version, cfg.GitBase(), &cfg.Secret)
		if err != nil {
			fmt.Fprintf(os.Stderr, "    warning: %v\n", err)
			continue
		}
		codeDocs = append(codeDocs, repoDocs...)
	}
	// Include Go source from telco-reference (if any).
	for _, d := range telcoDocs {
		if d.Metadata["type"] == "go" {
			codeDocs = append(codeDocs, d)
		}
	}
	fmt.Fprintf(os.Stderr, "  total code documents: %d\n", len(codeDocs))

	// Build known issues.
	knownIssues := BuildKnownIssues()
	fmt.Fprintf(os.Stderr, "  known issues: %d\n", len(knownIssues))

	// Build manifests: YAML from telco-reference + CRD files from repos.
	var manifestDocs []rag.Document

	// Telco YAML files (already loaded in telcoDocs, but we want only
	// YAML-type docs for the manifests collection).
	for _, d := range telcoDocs {
		if d.Metadata["type"] == "yaml" {
			manifestDocs = append(manifestDocs, d)
		}
	}

	// CRD files from repos.
	for _, repo := range cfg.OpenShift.Repos {
		repoDir := filepath.Join(reposDir, repo)
		crdDirs := []string{
			filepath.Join(repoDir, "manifests"),
			filepath.Join(repoDir, "config", "crd"),
			filepath.Join(repoDir, "deploy"),
		}
		for _, crdDir := range crdDirs {
			if _, err := os.Stat(crdDir); err != nil {
				continue
			}
			crdDocs, err := LoadYAMLDir(crdDir)
			if err != nil {
				continue
			}
			for i := range crdDocs {
				crdDocs[i].Metadata["repo"] = repo
			}
			manifestDocs = append(manifestDocs, crdDocs...)
		}
	}
	fmt.Fprintf(os.Stderr, "  manifest documents: %d\n", len(manifestDocs))

	// Populate collections.
	collections := []struct {
		name rag.Collection
		docs []rag.Document
	}{
		{rag.CollDocs, ocpDocs},
		{rag.CollCode, codeDocs},
		{rag.CollTelco, telcoDocs},
		{rag.CollKnownIssues, knownIssues},
		{rag.CollManifests, manifestDocs},
		{rag.CollACMDocs, acmDocs},
	}

	for _, c := range collections {
		fmt.Fprintf(os.Stderr, "  indexing %s (%d docs) ...\n", c.name, len(c.docs))
		if err := store.Reset(c.name); err != nil {
			fmt.Fprintf(os.Stderr, "    warning: reset %s: %v\n", c.name, err)
		}
		if len(c.docs) > 0 {
			if err := store.AddDocuments(ctx, c.name, c.docs); err != nil {
				fmt.Fprintf(os.Stderr, "    warning: add documents to %s: %v\n", c.name, err)
			}
		}
	}

	// Step 7: Save freshness metadata.
	fmt.Fprintf(os.Stderr, "\n[6/7] Saving ingestion metadata ...\n")
	if err := rag.SaveIngestMeta(cfg.DataDir, cfg.Freshness.MetaFile, rag.AllCollections, repoCommits); err != nil {
		return fmt.Errorf("save ingest meta: %w", err)
	}

	fmt.Fprintf(os.Stderr, "\n=== Ingestion complete ===\n")
	return nil
}

// repoGitHubPath resolves a config repos[] entry to its full "org/repo" GitHub
// path. Entries already containing a "/" (e.g. "red-hat-storage/odf-operator")
// are used as-is, so operators living outside the openshift/ org (ODF, ACM,
// TALM, GitOps, etc.) can be ingested too. Bare names (e.g. "cluster-etcd-operator")
// keep the historical default of the openshift/ org for backward compatibility.
func repoGitHubPath(repo string) string {
	if strings.Contains(repo, "/") {
		return repo
	}
	return "openshift/" + repo
}

// orDefault returns s, or fallback if s is empty.
func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// cloneAllRepos clones or updates all configured OpenShift repos concurrently,
// limited to cfg.Ingestion.MaxParallelClones goroutines. Returns a map of
// repo -> HEAD commit hash.
func cloneAllRepos(ctx context.Context, cfg *rag.Config, reposDir, branch string) (map[string]string, error) {
	commits := make(map[string]string)
	mu := make(chan struct{}, 1) // protects commits map

	maxParallel := cfg.Ingestion.MaxParallelClones
	if maxParallel <= 0 {
		maxParallel = 4
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxParallel)

	for _, repo := range cfg.OpenShift.Repos {
		g.Go(func() error {
			repoDir := filepath.Join(reposDir, repo)
			timeout := cfg.Ingestion.GitTimeout.Duration

			if _, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil {
				// Already cloned — pull.
				fmt.Fprintf(os.Stderr, "  updating %s ...\n", repo)
				cmdCtx, cancel := context.WithTimeout(gctx, timeout)
				defer cancel()
				cmd := exec.CommandContext(cmdCtx, "git", "pull", "--ff-only")
				cmd.Dir = repoDir
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					fmt.Fprintf(os.Stderr, "  warning: git pull %s: %v\n", repo, err)
				}
			} else {
				// Clone fresh. External (org/repo) entries live outside the
				// openshift/ org and follow their own project-specific branch
				// naming (e.g. ACM release-2.x, GitOps release-1.x) that has
				// no relation to the OCP release branch, so clone their
				// default branch instead of forcing the OCP branch on them.
				cloneBranch := branch
				if strings.Contains(repo, "/") {
					cloneBranch = ""
				}
				fmt.Fprintf(os.Stderr, "  cloning %s (branch: %s) ...\n", repo, orDefault(cloneBranch, "default"))
				cmdCtx, cancel := context.WithTimeout(gctx, timeout)
				defer cancel()
				url := cfg.RepoURL(repoGitHubPath(repo))
				cloneArgs := []string{"clone", "--depth=1"}
				if cloneBranch != "" {
					cloneArgs = append(cloneArgs, "--branch", cloneBranch)
				}
				cloneArgs = append(cloneArgs, url, repoDir)
				cmd := exec.CommandContext(cmdCtx, "git", cloneArgs...)
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					fmt.Fprintf(os.Stderr, "  warning: git clone %s: %v\n", repo, err)
					return nil // don't fail the entire pipeline
				}
			}

			// Read HEAD commit.
			headCmd := exec.Command("git", "rev-parse", "HEAD")
			headCmd.Dir = repoDir
			out, err := headCmd.Output()
			if err == nil {
				commit := strings.TrimSpace(string(out))
				mu <- struct{}{}
				commits[repo] = commit
				<-mu
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return commits, err
	}
	return commits, nil
}
