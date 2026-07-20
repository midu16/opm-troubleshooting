package openshift

import (
	"strings"

	"github.com/midu16/opm-troubleshooting/internal/codeanalysis"
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
