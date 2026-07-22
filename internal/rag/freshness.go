package rag

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type IngestMeta struct {
	Timestamp   string            `json:"timestamp"`
	Collections []string          `json:"collections"`
	RepoCommits map[string]string `json:"repo_commits"`
}

func SaveIngestMeta(dataDir, metaFile string, collections []Collection, repoCommits map[string]string) error {
	meta := IngestMeta{
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Collections: make([]string, len(collections)),
		RepoCommits: repoCommits,
	}
	for i, c := range collections {
		meta.Collections[i] = string(c)
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dataDir, metaFile), data, 0o600)
}

func LoadIngestMeta(dataDir, metaFile string) (*IngestMeta, error) {
	data, err := os.ReadFile(filepath.Join(dataDir, metaFile))
	if err != nil {
		return nil, err
	}

	var meta IngestMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func CheckFreshness(dataDir, metaFile string) (*FreshnessStatus, error) {
	meta, err := LoadIngestMeta(dataDir, metaFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &FreshnessStatus{
				Fresh:   false,
				Message: "No ingestion metadata found — run ocp-rag-ingest to populate the knowledge base",
			}, nil
		}
		return nil, err
	}

	status := &FreshnessStatus{
		Fresh:         true,
		IngestedAt:    meta.Timestamp,
		IngestCommits: meta.RepoCommits,
	}

	stale := 0
	for repo, commit := range meta.RepoCommits {
		repoDir := filepath.Join(dataDir, "repos", repo)
		if _, err := os.Stat(repoDir); err != nil {
			continue
		}
		headCommit, err := gitHeadCommit(repoDir)
		if err != nil {
			continue
		}
		if headCommit != commit {
			stale++
		}
	}

	if stale > 0 {
		status.Fresh = false
		status.Message = fmt.Sprintf("%d repos have new commits since last ingestion at %s", stale, meta.Timestamp)
	} else {
		status.Message = fmt.Sprintf("Knowledge base is up to date (ingested at %s)", meta.Timestamp)
	}

	return status, nil
}

func gitHeadCommit(repoDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
