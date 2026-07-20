package learning

import (
	"fmt"
	"strings"

	"github.com/midu16/opm-troubleshooting/internal/healthcheck"
	"github.com/midu16/opm-troubleshooting/internal/metadata"
	"github.com/midu16/opm-troubleshooting/internal/mustgather"
	"github.com/midu16/opm-troubleshooting/internal/noise"
	"github.com/midu16/opm-troubleshooting/internal/rca"
)

// SymptomInput holds the data needed to build a fingerprint.
type SymptomInput struct {
	Operator      mustgather.OperatorState
	HealthReport  *healthcheck.Report
	InfraReport   *healthcheck.Report
	NoiseReport   *noise.FilterReport
	RCAPatterns   []rca.PatternMatch
}

// BuildFingerprint creates an issue fingerprint from analysis results.
func BuildFingerprint(input SymptomInput) metadata.Fingerprint {
	symptoms := NormalizeSymptoms(input)
	hash := metadata.ComputeSymptomHash(symptoms)

	classification := ""
	if input.Operator.FailureReason != "" {
		failLower := strings.ToLower(input.Operator.FailureReason)
		if strings.Contains(failLower, "missing crd") || strings.Contains(failLower, "rbac") ||
			strings.Contains(failLower, "forbidden") || strings.Contains(failLower, "configmap") {
			classification = "configuration"
		}
	}

	return metadata.Fingerprint{
		SymptomHash:    hash,
		Operator:       input.Operator.PackageName,
		Symptoms:       symptoms,
		Classification: classification,
		Confidence:     computeConfidence(input),
	}
}

// NormalizeSymptoms extracts a stable, sorted list of symptoms from analysis results.
func NormalizeSymptoms(input SymptomInput) []string {
	var symptoms []string

	if input.Operator.State != "" && input.Operator.State != "AtLatestKnown" {
		symptoms = append(symptoms, fmt.Sprintf("state:%s", strings.ToLower(input.Operator.State)))
	}

	if input.Operator.FailureReason != "" {
		keywords := extractFailureKeywords(input.Operator.FailureReason)
		for _, kw := range keywords {
			symptoms = append(symptoms, fmt.Sprintf("failure:%s", kw))
		}
	}

	if input.HealthReport != nil {
		for _, dim := range input.HealthReport.Dimensions {
			if dim.Status == healthcheck.StatusFail {
				symptoms = append(symptoms, fmt.Sprintf("health:%s:fail", strings.ToLower(dim.Name)))
			}
		}
	}

	if input.InfraReport != nil {
		for _, dim := range input.InfraReport.Dimensions {
			if dim.Status == healthcheck.StatusFail {
				symptoms = append(symptoms, fmt.Sprintf("infra:%s:fail", strings.ToLower(dim.Name)))
			}
		}
	}

	for _, p := range input.RCAPatterns {
		if p.Confidence >= 0.5 {
			symptoms = append(symptoms, fmt.Sprintf("pattern:%s", strings.ToLower(string(p.Pattern))))
		}
	}

	return symptoms
}

func extractFailureKeywords(reason string) []string {
	words := strings.Fields(strings.ToLower(reason))
	var keywords []string
	for _, w := range words {
		w = strings.Trim(w, ".,;:!?\"'()[]{}=<>")
		if len(w) >= 4 && !isCommonWord(w) {
			keywords = append(keywords, w)
		}
		if len(keywords) >= 6 {
			break
		}
	}
	return keywords
}

func isCommonWord(w string) bool {
	common := map[string]bool{
		"the": true, "and": true, "for": true, "not": true, "but": true,
		"with": true, "this": true, "that": true, "from": true, "have": true,
		"has": true, "was": true, "are": true, "been": true, "were": true,
	}
	return common[w]
}

func computeConfidence(input SymptomInput) float64 {
	confidence := 0.3
	if input.Operator.FailureReason != "" {
		confidence += 0.2
	}
	if input.HealthReport != nil && input.HealthReport.Failed > 0 {
		confidence += 0.2
	}
	if len(input.RCAPatterns) > 0 {
		confidence += 0.2
	}
	if input.NoiseReport != nil && input.NoiseReport.RealIssues > 0 {
		confidence += 0.1
	}
	if confidence > 1.0 {
		confidence = 1.0
	}
	return confidence
}
