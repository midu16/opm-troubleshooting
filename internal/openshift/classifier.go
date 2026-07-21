package openshift

import (
	"fmt"
	"strings"

	"github.com/midu16/opm-troubleshooting/internal/codeanalysis"
	"github.com/midu16/opm-troubleshooting/internal/gitdelta"
	"github.com/midu16/opm-troubleshooting/internal/healthcheck"
)

// Classification result types.
const (
	ClassCodeBug        = "code_bug"
	ClassConfiguration  = "configuration"
	ClassInfrastructure = "infrastructure"
	ClassUnknown        = "unknown"
)

// Classification describes whether an issue is a code bug, configuration problem, or infra issue.
type Classification struct {
	Type       string   `json:"type"`
	Confidence float64  `json:"confidence"`
	Evidence   []string `json:"evidence"`
}

// Classify determines whether an operator issue is a code bug, configuration, or infrastructure problem.
func Classify(codeMatches []codeanalysis.Match, failureReason string, infraReport *healthcheck.Report) Classification {
	result := Classification{
		Type:     ClassUnknown,
		Evidence: make([]string, 0),
	}

	hasCodeMatches := len(codeMatches) > 0
	hasInfraFailures := infraReport != nil && infraReport.Failed > 0
	failureLower := strings.ToLower(failureReason)

	isConfigRelated := containsAny(failureLower, configKeywords)
	isInfraRelated := containsAny(failureLower, infraKeywords)

	if hasCodeMatches {
		result.Type = ClassCodeBug
		result.Confidence = 0.8
		result.Evidence = append(result.Evidence, "Error string found in operator source code")
		for _, m := range codeMatches {
			if len(result.Evidence) < 5 {
				result.Evidence = append(result.Evidence, m.FilePath+": "+truncate(m.LineContent, 80))
			}
		}

		if isConfigRelated {
			result.Confidence = 0.6
			result.Evidence = append(result.Evidence, "Error also relates to configuration — could be validation code rather than a bug")
		}
		return result
	}

	if isConfigRelated && !hasInfraFailures {
		result.Type = ClassConfiguration
		result.Confidence = 0.7
		result.Evidence = append(result.Evidence, "Error relates to configuration (RBAC, CRDs, resource specs)")
		result.Evidence = append(result.Evidence, "No matching error string in operator source code")
		return result
	}

	if hasInfraFailures || isInfraRelated {
		result.Type = ClassInfrastructure
		result.Confidence = 0.7
		result.Evidence = append(result.Evidence, "Error relates to infrastructure (nodes, network, storage)")
		if infraReport != nil {
			for _, dim := range infraReport.Dimensions {
				if dim.Status == healthcheck.StatusFail && len(result.Evidence) < 5 {
					result.Evidence = append(result.Evidence, "Infra failure: "+dim.Name+" — "+dim.Summary)
				}
			}
		}
		return result
	}

	if !hasCodeMatches && failureReason != "" {
		result.Type = ClassConfiguration
		result.Confidence = 0.4
		result.Evidence = append(result.Evidence, "No code match found — likely a configuration or environmental issue")
	}

	return result
}

var configKeywords = []string{
	"missing crd", "crd not found", "rbac", "forbidden", "unauthorized",
	"permission denied", "service account", "clusterrole", "role binding",
	"configmap not found", "secret not found", "namespace not found",
	"invalid spec", "validation failed", "immutable field", "required field",
	"unsupported value", "unknown field", "apiversion not found",
	"no matches for kind", "subscription", "installplan",
}

var infraKeywords = []string{
	"node not ready", "disk pressure", "memory pressure", "pid pressure",
	"network unreachable", "connection refused", "connection timed out",
	"i/o timeout", "no route to host", "dns resolution failed",
	"storage class", "persistent volume", "pvc pending", "mount failed",
	"image pull", "errimagepull", "imagepullbackoff",
	"oom", "out of memory", "evicted", "taint", "unschedulable",
	"certificate expired", "x509", "tls handshake",
}

func containsAny(text string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

var codeChangeKeywords = []string{
	"fix", "bug", "regression", "panic", "crash", "nil pointer",
	"goroutine", "deadlock", "race condition", "segfault",
}

var configChangeKeywords = []string{
	"config", "rbac", "crd", "permission", "policy", "role",
	"serviceaccount", "secret", "configmap", "annotation",
}

// ClassifyEnhanced determines issue classification using additional evidence
// from commit deltas, bundle metadata, and commit-pinned analysis.
func ClassifyEnhanced(
	codeMatches []codeanalysis.Match,
	failureReason string,
	infraReport *healthcheck.Report,
	commitDelta *gitdelta.CommitDelta,
	bundleCommit string,
	commitPinned bool,
) Classification {
	result := Classification{
		Type:     ClassUnknown,
		Evidence: make([]string, 0),
	}

	hasCodeMatches := len(codeMatches) > 0
	hasInfraFailures := infraReport != nil && infraReport.Failed > 0
	failureLower := strings.ToLower(failureReason)

	isConfigRelated := containsAny(failureLower, configKeywords)
	isInfraRelated := containsAny(failureLower, infraKeywords)

	// Base keyword score (weight 0.4)
	keywordScore := 0.0
	keywordType := ClassUnknown

	if hasCodeMatches {
		keywordScore = 1.0
		keywordType = ClassCodeBug
		result.Evidence = append(result.Evidence, "Error string found in operator source code")
		for _, m := range codeMatches {
			if len(result.Evidence) < 5 {
				result.Evidence = append(result.Evidence, m.FilePath+": "+truncate(m.LineContent, 80))
			}
		}
		if isConfigRelated {
			keywordScore = 0.6
			result.Evidence = append(result.Evidence, "Error also relates to configuration — could be validation code rather than a bug")
		}
	} else if isConfigRelated && !hasInfraFailures {
		keywordScore = 0.7
		keywordType = ClassConfiguration
		result.Evidence = append(result.Evidence, "Error relates to configuration (RBAC, CRDs, resource specs)")
	} else if hasInfraFailures || isInfraRelated {
		keywordScore = 0.7
		keywordType = ClassInfrastructure
		result.Evidence = append(result.Evidence, "Error relates to infrastructure (nodes, network, storage)")
		if infraReport != nil {
			for _, dim := range infraReport.Dimensions {
				if dim.Status == healthcheck.StatusFail && len(result.Evidence) < 5 {
					result.Evidence = append(result.Evidence, "Infra failure: "+dim.Name+" — "+dim.Summary)
				}
			}
		}
	} else if failureReason != "" {
		keywordScore = 0.3
		keywordType = ClassConfiguration
		result.Evidence = append(result.Evidence, "No code match found — likely a configuration or environmental issue")
	}

	// Code match score (weight 0.3)
	codeScore := 0.0
	if hasCodeMatches {
		codeScore = 0.7
		if len(codeMatches) >= 3 {
			codeScore = 1.0
			result.Evidence = append(result.Evidence, fmt.Sprintf("Strong code evidence: %d matches found", len(codeMatches)))
		}
	}

	// Commit delta score (weight 0.2)
	deltaScore := 0.0
	deltaType := ""
	if commitDelta != nil && len(commitDelta.FilesChanged) > 0 {
		codeFiles, configFiles := classifyFileChanges(commitDelta.FilesChanged)
		total := codeFiles + configFiles

		if total > 0 {
			if codeFiles > configFiles {
				deltaScore = float64(codeFiles) / float64(total)
				deltaType = ClassCodeBug
				result.Evidence = append(result.Evidence,
					fmt.Sprintf("Git delta: %d/%d changed files are in code directories", codeFiles, len(commitDelta.FilesChanged)))
			} else if configFiles > codeFiles {
				deltaScore = float64(configFiles) / float64(total)
				deltaType = ClassConfiguration
				result.Evidence = append(result.Evidence,
					fmt.Sprintf("Git delta: %d/%d changed files are in config directories", configFiles, len(commitDelta.FilesChanged)))
			}
		}

		if commitDelta.DiffSummary != "" {
			summaryLower := strings.ToLower(commitDelta.DiffSummary)
			if containsAny(summaryLower, codeChangeKeywords) {
				if deltaType != ClassCodeBug {
					deltaScore = (deltaScore + 0.5) / 2
				}
				deltaType = ClassCodeBug
				result.Evidence = append(result.Evidence, "Commit messages reference code-related changes (fix/bug/regression)")
			}
			if containsAny(summaryLower, configChangeKeywords) {
				if deltaType != ClassConfiguration {
					deltaScore = (deltaScore + 0.5) / 2
				}
				deltaType = ClassConfiguration
				result.Evidence = append(result.Evidence, "Commit messages reference configuration changes")
			}
		}
	}

	// Commit pinning bonus (weight 0.1)
	pinScore := 0.0
	if commitPinned {
		pinScore = 1.0
		result.Evidence = append(result.Evidence, "Analysis pinned to deployed commit — high fidelity")
	}

	// Determine final type: code matches take priority, then delta, then keywords
	if keywordType == ClassCodeBug {
		result.Type = ClassCodeBug
	} else if deltaType == ClassCodeBug && deltaScore > 0.5 {
		result.Type = ClassCodeBug
	} else if keywordType == ClassInfrastructure {
		result.Type = ClassInfrastructure
	} else if keywordType == ClassConfiguration || deltaType == ClassConfiguration {
		result.Type = ClassConfiguration
	} else if keywordType != ClassUnknown {
		result.Type = keywordType
	}

	// Compute weighted confidence
	confidence := keywordScore*0.4 + codeScore*0.3 + deltaScore*0.2 + pinScore*0.1
	if confidence > 1.0 {
		confidence = 1.0
	}
	result.Confidence = confidence

	return result
}

// classifyFileChanges categorizes changed files as code or config directory files.
func classifyFileChanges(files []string) (codeFiles, configFiles int) {
	for _, f := range files {
		lower := strings.ToLower(f)
		switch {
		case strings.HasPrefix(lower, "config/"),
			strings.HasPrefix(lower, "deploy/"),
			strings.HasPrefix(lower, "manifests/"),
			strings.Contains(lower, "crds/"),
			strings.Contains(lower, "rbac/"):
			configFiles++
		case strings.HasPrefix(lower, "pkg/"),
			strings.HasPrefix(lower, "cmd/"),
			strings.HasPrefix(lower, "internal/"),
			strings.HasPrefix(lower, "controllers/"):
			codeFiles++
		}
	}
	return
}
