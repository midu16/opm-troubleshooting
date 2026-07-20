package openshift

import (
	"context"
	"fmt"
	"strings"

	"github.com/midu16/opm-troubleshooting/internal/codeanalysis"
	"github.com/midu16/opm-troubleshooting/internal/healthcheck"
)

// Correlation holds the full result of correlating an operator issue against its source repo.
type Correlation struct {
	Operator       string            `json:"operator"`
	RepoPath       string            `json:"repo_path"`
	RepoURL        string            `json:"repo_url"`
	Classification Classification    `json:"classification"`
	CodeMatches    []codeanalysis.Match `json:"code_matches,omitempty"`
	GitHubIssues   []GitHubIssue     `json:"github_issues,omitempty"`
	RecentCommits  []CommitInfo      `json:"recent_commits,omitempty"`
	Evidence       []string          `json:"evidence"`
	Recommendation string            `json:"recommendation"`
}

// CorrelateConfig holds parameters for the correlation operation.
type CorrelateConfig struct {
	CacheDir       string
	InfraReport    *healthcheck.Report
	SearchDays     int
	SkipClone      bool
	SkipGitHub     bool
}

// Correlate runs the full pipeline: resolve repo → clone → search source → search issues → classify.
func Correlate(ctx context.Context, operator, failureReason string, cfg CorrelateConfig) (*Correlation, error) {
	info, ok := LookupRepo(operator)
	if !ok {
		return nil, fmt.Errorf("no registered repo for operator %q", operator)
	}

	result := &Correlation{
		Operator: operator,
		RepoPath: info.Repo,
		RepoURL:  RepoURL(info.Repo),
		Evidence: make([]string, 0),
	}

	searchDays := cfg.SearchDays
	if searchDays <= 0 {
		searchDays = 90
	}

	patterns := codeanalysis.PatternsFromFailure(failureReason, nil)

	if !cfg.SkipClone && failureReason != "" && len(patterns) > 0 {
		localPath, err := CloneOrUpdate(ctx, info.Repo, cfg.CacheDir)
		if err != nil {
			result.Evidence = append(result.Evidence, fmt.Sprintf("Clone failed: %v", err))
		} else {
			caResult, err := codeanalysis.Analyze(ctx, codeanalysis.Config{
				RepoPath:       localPath,
				SearchPatterns: patterns,
			})
			if err == nil && caResult != nil {
				result.CodeMatches = caResult.Matches
				if len(caResult.Matches) > 0 {
					result.Evidence = append(result.Evidence,
						fmt.Sprintf("Found %d code match(es) in %s", len(caResult.Matches), info.Repo))
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
	}

	if !cfg.SkipGitHub && failureReason != "" {
		searchTerms := ExtractSearchTerms(failureReason)
		issues, err := SearchIssues(ctx, info.Repo, searchTerms)
		if err == nil {
			result.GitHubIssues = issues
			if len(issues) > 0 {
				result.Evidence = append(result.Evidence,
					fmt.Sprintf("Found %d related GitHub issue(s)", len(issues)))
			}
		}
	}

	result.Classification = Classify(result.CodeMatches, failureReason, cfg.InfraReport)
	result.Recommendation = buildRecommendation(result)

	return result, nil
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
