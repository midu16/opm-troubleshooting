package learning

import (
	"time"

	"github.com/midu16/opm-troubleshooting/internal/metadata"
)

// SimilarIssue pairs a past fingerprint with its similarity to the current issue.
type SimilarIssue struct {
	SymptomHash    string
	Operator       string
	Symptoms       []string
	RootCause      string
	Classification string
	Resolution     string
	Similarity     float64
	HitCount       int
	LastSeen       time.Time
}

// Insights aggregates learning data for RCA report sections.
type Insights struct {
	SimilarIssues []SimilarIssue
	FrameStats    []metadata.FrameStats
	TopPatterns   []metadata.PatternStat
	ExactMatch    *SimilarIssue
}

// FindSimilarIssues searches the metadata store for fingerprints matching the current symptoms.
func FindSimilarIssues(store *metadata.MetadataStore, fp metadata.Fingerprint) ([]SimilarIssue, error) {
	results := make([]SimilarIssue, 0, 8)

	exact, err := store.FindByHash(fp.SymptomHash)
	if err != nil {
		return nil, err
	}
	if exact != nil {
		results = append(results, SimilarIssue{
			SymptomHash:    exact.SymptomHash,
			Operator:       exact.Operator,
			Symptoms:       exact.Symptoms,
			RootCause:      exact.RootCause,
			Classification: exact.Classification,
			Resolution:     exact.Resolution,
			Similarity:     1.0,
			HitCount:       exact.HitCount,
			LastSeen:       exact.LastSeen,
		})
	}

	operatorMatches, err := store.FindSimilarForOperator(fp.Operator, fp.Symptoms, 0.4)
	if err != nil {
		return results, err
	}
	for _, m := range operatorMatches {
		if m.Fingerprint.SymptomHash == fp.SymptomHash {
			continue
		}
		results = append(results, SimilarIssue{
			SymptomHash:    m.Fingerprint.SymptomHash,
			Operator:       m.Fingerprint.Operator,
			Symptoms:       m.Fingerprint.Symptoms,
			RootCause:      m.Fingerprint.RootCause,
			Classification: m.Fingerprint.Classification,
			Resolution:     m.Fingerprint.Resolution,
			Similarity:     m.Similarity,
			HitCount:       m.Fingerprint.HitCount,
			LastSeen:       m.Fingerprint.LastSeen,
		})
	}

	globalMatches, err := store.FindSimilar(fp.Symptoms, 0.6)
	if err != nil {
		return results, err
	}
	seen := make(map[string]bool)
	for _, r := range results {
		seen[r.SymptomHash] = true
	}
	for _, m := range globalMatches {
		if seen[m.Fingerprint.SymptomHash] {
			continue
		}
		seen[m.Fingerprint.SymptomHash] = true
		results = append(results, SimilarIssue{
			SymptomHash:    m.Fingerprint.SymptomHash,
			Operator:       m.Fingerprint.Operator,
			Symptoms:       m.Fingerprint.Symptoms,
			RootCause:      m.Fingerprint.RootCause,
			Classification: m.Fingerprint.Classification,
			Resolution:     m.Fingerprint.Resolution,
			Similarity:     m.Similarity,
			HitCount:       m.Fingerprint.HitCount,
			LastSeen:       m.Fingerprint.LastSeen,
		})
	}

	return results, nil
}

// BuildInsights aggregates learning data for the RCA report.
func BuildInsights(store *metadata.MetadataStore, operator string, similarIssues []SimilarIssue) (*Insights, error) {
	insights := &Insights{
		SimilarIssues: similarIssues,
	}

	if len(similarIssues) > 0 && similarIssues[0].Similarity == 1.0 {
		insights.ExactMatch = &similarIssues[0]
	}

	frameStats, err := store.GetFrameStatsForOperator(operator)
	if err == nil {
		insights.FrameStats = frameStats
	}

	patterns, err := store.GetPatternFrequency(operator)
	if err == nil {
		insights.TopPatterns = patterns
	}

	return insights, nil
}
