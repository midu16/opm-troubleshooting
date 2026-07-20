package learning

import (
	"github.com/midu16/opm-troubleshooting/internal/adhd"
	"github.com/midu16/opm-troubleshooting/internal/metadata"
)

// ComputeBoostFactors retrieves per-frame accuracy-based boost factors from the metadata store.
func ComputeBoostFactors(store *metadata.MetadataStore, operator string) (map[string]float64, error) {
	return store.GetBoostFactors(operator)
}

// ApplyBoosts adjusts hypothesis scores based on historical frame accuracy.
func ApplyBoosts(hypotheses []adhd.Hypothesis, boosts map[string]float64) []adhd.Hypothesis {
	if len(boosts) == 0 {
		return hypotheses
	}
	result := make([]adhd.Hypothesis, len(hypotheses))
	copy(result, hypotheses)

	for i := range result {
		factor, ok := boosts[result[i].FrameID]
		if !ok {
			continue
		}
		result[i].Score.Likelihood *= factor
		result[i].Score.Total = computeWeightedTotal(result[i].Score)
	}
	return result
}

func computeWeightedTotal(s adhd.Score) float64 {
	return s.Likelihood*0.4 + s.Impact*0.25 + s.Evidence*0.35
}
