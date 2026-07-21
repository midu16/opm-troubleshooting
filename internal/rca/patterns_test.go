package rca

import (
	"testing"
)

func TestPatternDetector_DetectPatterns(t *testing.T) {
	detector := NewPatternDetector()

	tests := []struct {
		name             string
		failureSymptoms  string
		expectedPatterns []Pattern
		minConfidence    float64
	}{
		{
			name:             "Nil pointer / Missing Guard",
			failureSymptoms:  "panic: nil pointer dereference in operator reconcile loop",
			expectedPatterns: []Pattern{PatternMissingGuard},
			minConfidence:    0.8,
		},
		{
			name:             "Race condition with timing",
			failureSymptoms:  "Thread 1 started engine, Thread 2 (6ms later) stopped engine causing intermittent failures",
			expectedPatterns: []Pattern{PatternRaceCondition},
			minConfidence:    0.5,
		},
		{
			name:             "Asymmetry between phases",
			failureSymptoms:  "Mirror phase tolerates missing signatures, archive phase fatal error on same condition",
			expectedPatterns: []Pattern{PatternAsymmetry},
			minConfidence:    0.5,
		},
		{
			name:             "State divergence",
			failureSymptoms:  "ConfigMap state inconsistent between master and worker nodes, stale data detected",
			expectedPatterns: []Pattern{PatternStateDivergence},
			minConfidence:    0.6,
		},
		{
			name:             "Error swallowing",
			failureSymptoms:  "Error from API call silently ignored, not logged or returned to caller",
			expectedPatterns: []Pattern{PatternErrorSwallowing},
			minConfidence:    0.6,
		},
		{
			name:            "No patterns",
			failureSymptoms: "Operator deployed successfully",
			minConfidence:   0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := detector.DetectPatterns(tt.failureSymptoms)

			if tt.minConfidence == 0.0 {
				// Expect no high-confidence matches
				for _, match := range matches {
					if match.Confidence >= 0.5 {
						t.Errorf("Expected no high-confidence patterns, got %v with confidence %.2f",
							match.Pattern, match.Confidence)
					}
				}
				return
			}

			// Check if expected patterns were detected
			for _, expectedPattern := range tt.expectedPatterns {
				found := false
				for _, match := range matches {
					if match.Pattern == expectedPattern {
						found = true
						if match.Confidence < tt.minConfidence {
							t.Errorf("Pattern %v detected but confidence too low: got %.2f, want >= %.2f",
								expectedPattern, match.Confidence, tt.minConfidence)
						}
						if len(match.Evidence) == 0 {
							t.Errorf("Pattern %v detected but no evidence provided", expectedPattern)
						}
						if match.Description == "" {
							t.Errorf("Pattern %v detected but no description provided", expectedPattern)
						}
						break
					}
				}
				if !found {
					t.Errorf("Expected pattern %v not detected in matches: %v", expectedPattern, matches)
				}
			}
		})
	}
}

func TestGetRecommendations(t *testing.T) {
	tests := []struct {
		name        string
		pattern     Pattern
		context     string
		minRecs     int
		wantCritical bool
	}{
		{
			name:         "Missing Guard recommendations",
			pattern:      PatternMissingGuard,
			context:      "nil pointer",
			minRecs:      1,
			wantCritical: true,
		},
		{
			name:         "Race Condition recommendations",
			pattern:      PatternRaceCondition,
			context:      "concurrent access",
			minRecs:      2,
			wantCritical: true,
		},
		{
			name:         "Asymmetry recommendations",
			pattern:      PatternAsymmetry,
			context:      "phase difference",
			minRecs:      1,
			wantCritical: true,
		},
		{
			name:         "Unknown pattern",
			pattern:      Pattern("UNKNOWN"),
			context:      "",
			minRecs:      1,
			wantCritical: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recs := GetRecommendations(tt.pattern, tt.context)

			if len(recs) < tt.minRecs {
				t.Errorf("Expected at least %d recommendations, got %d", tt.minRecs, len(recs))
			}

			hasCritical := false
			for _, rec := range recs {
				if rec.Priority == 1 {
					hasCritical = true
				}
				if rec.Title == "" {
					t.Error("Recommendation has empty title")
				}
				if rec.Description == "" {
					t.Error("Recommendation has empty description")
				}
			}

			if tt.wantCritical && !hasCritical {
				t.Error("Expected at least one critical (priority 1) recommendation")
			}
		})
	}
}

func TestGetPatternDescription(t *testing.T) {
	tests := []struct {
		pattern Pattern
		want    string
	}{
		{PatternAsymmetry, "Different behavior in similar contexts"},
		{PatternMissingGuard, "Missing null/error checks"},
		{PatternRaceCondition, "Timing-dependent failures"},
		{Pattern("UNKNOWN"), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(string(tt.pattern), func(t *testing.T) {
			desc := getPatternDescription(tt.pattern)
			if desc == "" {
				t.Error("Empty description returned")
			}
			if tt.pattern != "UNKNOWN" && !contains(desc, tt.want) {
				t.Errorf("Description %q does not contain expected text %q", desc, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		 findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
