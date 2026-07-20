package adhd

import "strings"

// Score weights for the total calculation.
const (
	weightLikelihood = 0.40
	weightImpact     = 0.25
	weightEvidence   = 0.35
)

// CalculateTotal computes the weighted total score.
// Weights: Likelihood 0.40, Impact 0.25, Evidence 0.35.
func CalculateTotal(s *Score) {
	s.Total = s.Likelihood*weightLikelihood + s.Impact*weightImpact + s.Evidence*weightEvidence
}

// trapPattern pairs a text pattern with the reason it is a diagnostic trap.
type trapPattern struct {
	Pattern string
	Reason  string
}

// trapPatterns lists common OpenShift troubleshooting traps -- hypotheses that
// suggest mitigation (hiding the symptom) rather than diagnosis (finding the
// root cause).
var trapPatterns = []trapPattern{
	{"restart the pod", "Mitigation, not diagnosis — the root cause will return"},
	{"restart the node", "Mitigation, not diagnosis — hiding a deeper issue"},
	{"delete and recreate", "Masked symptom — fails to explain WHY it broke"},
	{"increase resource limits", "Treating the symptom — something is consuming more than expected"},
	{"scale up", "Capacity workaround — doesn't explain the resource demand spike"},
	{"disable the webhook", "Removes safety — the webhook exists for a reason"},
	{"force delete", "Loses state — root cause hidden by removing evidence"},
}

// DetectTraps examines a hypothesis text and marks its score with a trap
// reason if the hypothesis matches a known trap pattern. Returns true if a
// trap was detected.
func DetectTraps(h *Hypothesis) bool {
	lower := strings.ToLower(h.Text + " " + h.Rationale)
	for _, tp := range trapPatterns {
		if strings.Contains(lower, tp.Pattern) {
			h.Score.Trap = tp.Reason
			return true
		}
	}
	return false
}

// DetectTrapsAll runs trap detection over a slice of hypotheses and returns
// only those flagged as traps.
func DetectTrapsAll(hypotheses []Hypothesis) []Hypothesis {
	var traps []Hypothesis
	for i := range hypotheses {
		if DetectTraps(&hypotheses[i]) {
			traps = append(traps, hypotheses[i])
		}
	}
	return traps
}
