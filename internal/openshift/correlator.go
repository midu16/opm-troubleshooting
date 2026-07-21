package openshift

import (
	"context"
	"fmt"
	"strings"

	"github.com/midu16/opm-troubleshooting/internal/codeanalysis"
	"github.com/midu16/opm-troubleshooting/internal/gitdelta"
	"github.com/midu16/opm-troubleshooting/internal/healthcheck"
	"github.com/midu16/opm-troubleshooting/internal/imageinspect"
)

// Correlation holds the full result of correlating an operator issue against its source repo.
type Correlation struct {
	Operator        string               `json:"operator"`
	RepoPath        string               `json:"repo_path"`
	RepoURL         string               `json:"repo_url"`
	Classification  Classification       `json:"classification"`
	CodeMatches     []codeanalysis.Match  `json:"code_matches,omitempty"`
	GitHubIssues    []GitHubIssue        `json:"github_issues,omitempty"`
	RecentCommits   []CommitInfo         `json:"recent_commits,omitempty"`
	Evidence        []string             `json:"evidence"`
	Recommendation  string               `json:"recommendation"`
	BundleCommit    string               `json:"bundle_commit,omitempty"`
	RepoSource      string               `json:"repo_source"`
	RepoVerified    bool                 `json:"repo_verified"`
	CommitPinned    bool                 `json:"commit_pinned"`
	ConfidenceGrade string               `json:"confidence_grade"`
}

// CorrelateConfig holds parameters for the correlation operation.
type CorrelateConfig struct {
	CacheDir       string
	InfraReport    *healthcheck.Report
	SearchDays     int
	SkipClone      bool
	SkipGitHub     bool
	BundleInfo     *imageinspect.BundleInfo
	BundleRepoURLs []string
	CommitDelta    *gitdelta.CommitDelta
}

// Correlate runs the full pipeline: resolve repo → clone → search source → search issues → classify.
// It resolves the repo through a cascade: static registry → bundle labels → bundle CSV → inference.
func Correlate(ctx context.Context, operator, failureReason string, cfg CorrelateConfig) (*Correlation, error) {
	result := &Correlation{
		Operator: operator,
		Evidence: make([]string, 0),
	}

	// Multi-source repo resolution cascade
	repoPath, repoSource := resolveRepo(operator, cfg)
	result.RepoSource = repoSource

	if repoPath == "" {
		result.ConfidenceGrade = "F"
		result.Classification = Classification{
			Type:       ClassUnknown,
			Confidence: 0.1,
			Evidence:   []string{"No source repository could be resolved for this operator"},
		}
		result.Recommendation = fmt.Sprintf("Unable to locate source repository for %s. Check operator bundle metadata or register the repo in the static registry.", operator)
		return result, nil
	}

	result.RepoPath = repoPath
	result.RepoURL = RepoURL(repoPath)
	result.Evidence = append(result.Evidence, fmt.Sprintf("Repository resolved via %s: %s", repoSource, repoPath))

	// Verify repo exists on GitHub
	verification, err := VerifyRepo(ctx, repoPath)
	if err == nil && verification != nil {
		result.RepoVerified = verification.Verified && verification.Exists
		if verification.Exists {
			result.Evidence = append(result.Evidence, "Repository verified on GitHub")
		} else if verification.Verified {
			result.Evidence = append(result.Evidence, "Repository NOT found on GitHub")
		} else if verification.Error != "" {
			result.Evidence = append(result.Evidence, fmt.Sprintf("GitHub verification inconclusive: %s", verification.Error))
		}
	}

	searchDays := cfg.SearchDays
	if searchDays <= 0 {
		searchDays = 90
	}

	patterns := codeanalysis.PatternsFromFailure(failureReason, nil)

	// Determine commit for pinned analysis
	bundleCommit := ""
	if cfg.BundleInfo != nil && cfg.BundleInfo.Commit != "" {
		bundleCommit = cfg.BundleInfo.Commit
		result.BundleCommit = bundleCommit
	}

	if !cfg.SkipClone && failureReason != "" && len(patterns) > 0 {
		if bundleCommit != "" {
			// Commit-pinned analysis using CloneAndAnalyze
			repoURL := result.RepoURL
			caResult, err := codeanalysis.CloneAndAnalyze(ctx, repoURL, bundleCommit, patterns)
			if err == nil && caResult != nil {
				result.CodeMatches = caResult.Matches
				result.CommitPinned = true
				if len(caResult.Matches) > 0 {
					result.Evidence = append(result.Evidence,
						fmt.Sprintf("Found %d code match(es) at commit %s", len(caResult.Matches), bundleCommit[:minLen(len(bundleCommit), 8)]))
				}
				result.Evidence = append(result.Evidence, fmt.Sprintf("Code analysis pinned to deployed commit %s", bundleCommit[:minLen(len(bundleCommit), 12)]))
			} else if err != nil {
				result.Evidence = append(result.Evidence, fmt.Sprintf("Commit-pinned clone failed (%s), falling back to HEAD", bundleCommit[:minLen(len(bundleCommit), 8)]))
				// Fall back to HEAD-based analysis
				analyzeAtHEAD(ctx, result, repoPath, cfg.CacheDir, patterns, searchDays, failureReason)
			}
		} else {
			analyzeAtHEAD(ctx, result, repoPath, cfg.CacheDir, patterns, searchDays, failureReason)
		}
	}

	if !cfg.SkipGitHub && failureReason != "" {
		searchTerms := ExtractSearchTerms(failureReason)
		issues, err := SearchIssues(ctx, repoPath, searchTerms)
		if err == nil {
			result.GitHubIssues = issues
			if len(issues) > 0 {
				result.Evidence = append(result.Evidence,
					fmt.Sprintf("Found %d related GitHub issue(s)", len(issues)))
			}
		}
	}

	result.Classification = ClassifyEnhanced(
		result.CodeMatches, failureReason, cfg.InfraReport,
		cfg.CommitDelta, bundleCommit, result.CommitPinned,
	)
	result.ConfidenceGrade = computeGrade(result)
	result.Recommendation = buildRecommendation(result)

	return result, nil
}

// resolveRepo attempts to find the repo path through a cascade of sources.
func resolveRepo(operator string, cfg CorrelateConfig) (repoPath, source string) {
	// 1. Static registry
	if info, ok := LookupRepo(operator); ok {
		return info.Repo, "static_registry"
	}

	// 2. Bundle image labels
	if cfg.BundleInfo != nil && cfg.BundleInfo.URL != "" {
		if rp := ExtractRepoPath(cfg.BundleInfo.URL); rp != "" {
			return rp, "bundle_labels"
		}
	}

	// 3. Bundle CSV-derived URLs
	for _, u := range cfg.BundleRepoURLs {
		if rp := ExtractRepoPath(u); rp != "" {
			return rp, "bundle_csv"
		}
	}

	// 4. Infer from package name under openshift org
	inferred := "openshift/" + operator
	return inferred, "inferred"
}

// analyzeAtHEAD performs the original HEAD-based code analysis with CloneOrUpdate.
func analyzeAtHEAD(ctx context.Context, result *Correlation, repoPath, cacheDir string, patterns []string, searchDays int, failureReason string) {
	localPath, err := CloneOrUpdate(ctx, repoPath, cacheDir)
	if err != nil {
		result.Evidence = append(result.Evidence, fmt.Sprintf("Clone failed: %v", err))
		return
	}

	caResult, err := codeanalysis.Analyze(ctx, codeanalysis.Config{
		RepoPath:       localPath,
		SearchPatterns: patterns,
	})
	if err == nil && caResult != nil {
		result.CodeMatches = caResult.Matches
		if len(caResult.Matches) > 0 {
			result.Evidence = append(result.Evidence,
				fmt.Sprintf("Found %d code match(es) in %s (HEAD)", len(caResult.Matches), repoPath))
		}
	}

	keywords := extractKeywords(failureReason)
	commits, err := RecentCommits(ctx, localPath, searchDays, keywords)
	if err == nil {
		result.RecentCommits = commits
		if len(commits) > 0 {
			result.Evidence = append(result.Evidence,
				fmt.Sprintf("Found %d relevant commit(s) in last %d days", len(commits), searchDays))
		}
	}
}

// computeGrade assigns a letter grade based on repo source quality, verification, and evidence.
func computeGrade(c *Correlation) string {
	score := 0.0

	switch c.RepoSource {
	case "static_registry":
		score += 30
	case "bundle_labels":
		score += 25
	case "bundle_csv":
		score += 20
	case "inferred":
		score += 10
	}

	if c.RepoVerified {
		score += 20
	}
	if c.CommitPinned {
		score += 15
	}
	if len(c.CodeMatches) > 0 {
		score += 20
	}
	if len(c.CodeMatches) >= 3 {
		score += 5
	}

	score += c.Classification.Confidence * 10

	switch {
	case score >= 90:
		return "A"
	case score >= 75:
		return "B"
	case score >= 60:
		return "C"
	case score >= 40:
		return "D"
	default:
		return "F"
	}
}

func minLen(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func extractKeywords(failureReason string) []string {
	words := strings.Fields(failureReason)
	var keywords []string
	for _, w := range words {
		w = strings.Trim(w, ".,;:!?\"'()[]{}=<>")
		if len(w) >= 5 && !isStopWord(w) {
			keywords = append(keywords, strings.ToLower(w))
		}
	}
	if len(keywords) > 6 {
		keywords = keywords[:6]
	}
	return keywords
}

func isStopWord(w string) bool {
	stops := map[string]bool{
		"error": true, "failed": true, "found": true, "cannot": true,
		"could": true, "would": true, "should": true, "there": true,
		"which": true, "after": true, "before": true, "while": true,
		"during": true, "about": true, "their": true, "other": true,
	}
	return stops[strings.ToLower(w)]
}

func buildRecommendation(c *Correlation) string {
	switch c.Classification.Type {
	case ClassCodeBug:
		rec := fmt.Sprintf("This appears to be a code issue in %s.", c.RepoPath)
		if len(c.GitHubIssues) > 0 {
			rec += fmt.Sprintf(" Check %s for related issues.", c.GitHubIssues[0].URL)
		}
		if len(c.RecentCommits) > 0 {
			rec += " Recent commits may contain a fix — consider upgrading."
		}
		return rec

	case ClassConfiguration:
		return fmt.Sprintf("This appears to be a configuration issue. The operator (%s) is working as designed, but the cluster configuration needs correction. Review RBAC, CRDs, and resource specs.", c.RepoPath)

	case ClassInfrastructure:
		return "This appears to be an infrastructure issue. Check node health, network connectivity, storage, and platform-level resources before investigating the operator."

	default:
		return fmt.Sprintf("Unable to definitively classify. Review the operator source at https://github.com/%s and check cluster configuration.", c.RepoPath)
	}
}
