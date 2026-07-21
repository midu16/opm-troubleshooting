package adhd

import (
	"math"
	"testing"
)

func TestAllFrames(t *testing.T) {
	frames := AllFrames()

	if got := len(frames); got != 13 {
		t.Fatalf("AllFrames() returned %d frames, want 13", got)
	}

	seen := make(map[string]bool, len(frames))
	for i, f := range frames {
		if f.ID == "" {
			t.Errorf("frame[%d] has empty ID", i)
		}
		if f.Name == "" {
			t.Errorf("frame[%d] (%s) has empty Name", i, f.ID)
		}
		if len(f.Tags) == 0 {
			t.Errorf("frame[%d] (%s) has no Tags", i, f.ID)
		}
		if f.VantagePrompt == "" {
			t.Errorf("frame[%d] (%s) has empty VantagePrompt", i, f.ID)
		}
		if seen[f.ID] {
			t.Errorf("frame[%d] has duplicate ID %q", i, f.ID)
		}
		seen[f.ID] = true
	}

	// Verify AllFrames returns a copy, not the original slice.
	frames[0].ID = "mutated"
	original := AllFrames()
	if original[0].ID == "mutated" {
		t.Error("AllFrames() returned the backing slice, not a copy")
	}
}

func TestSelectFrames(t *testing.T) {
	tests := []struct {
		name      string
		tags      []string
		count     int
		wantCount int
		wantWild  bool // whether at least one "wild" frame is expected
	}{
		{
			name:      "count exceeds total returns all",
			tags:      []string{"infrastructure"},
			count:     20,
			wantCount: 13,
			wantWild:  true,
		},
		{
			name:      "zero count returns all",
			tags:      []string{"infrastructure"},
			count:     0,
			wantCount: 13,
			wantWild:  true,
		},
		{
			name:      "negative count returns all",
			tags:      nil,
			count:     -1,
			wantCount: 13,
			wantWild:  true,
		},
		{
			name:      "select 5 with infrastructure tag",
			tags:      []string{"infrastructure"},
			count:     5,
			wantCount: 5,
			wantWild:  true,
		},
		{
			name:      "select 2 with networking tag includes wild",
			tags:      []string{"networking"},
			count:     2,
			wantCount: 2,
			wantWild:  true,
		},
		{
			name:      "select 3 with security tag includes wild",
			tags:      []string{"security"},
			count:     3,
			wantCount: 3,
			wantWild:  true,
		},
		{
			name:      "select 1 frame",
			tags:      []string{"infrastructure"},
			count:     1,
			wantCount: 1,
			wantWild:  false, // with only 1 slot, no room to force a wild frame
		},
		{
			name:      "nil tags select count frames",
			tags:      nil,
			count:     4,
			wantCount: 4,
			wantWild:  true,
		},
		{
			name:      "empty tags select count frames",
			tags:      []string{},
			count:     6,
			wantCount: 6,
			wantWild:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selected := SelectFrames(tt.tags, tt.count)

			if got := len(selected); got != tt.wantCount {
				t.Errorf("SelectFrames(%v, %d) returned %d frames, want %d",
					tt.tags, tt.count, got, tt.wantCount)
			}

			if tt.wantWild {
				hasWild := false
				for _, f := range selected {
					if frameHasTag(f, "wild") {
						hasWild = true
						break
					}
				}
				if !hasWild {
					t.Errorf("SelectFrames(%v, %d) has no wild frame, want at least one",
						tt.tags, tt.count)
				}
			}

			// Verify no duplicate IDs in selection.
			seen := make(map[string]bool, len(selected))
			for _, f := range selected {
				if seen[f.ID] {
					t.Errorf("SelectFrames returned duplicate frame ID %q", f.ID)
				}
				seen[f.ID] = true
			}
		})
	}
}

func TestCalculateTotal(t *testing.T) {
	tests := []struct {
		name       string
		likelihood float64
		impact     float64
		evidence   float64
		wantTotal  float64
	}{
		{
			name:       "all ones",
			likelihood: 1.0,
			impact:     1.0,
			evidence:   1.0,
			wantTotal:  1.0, // 0.40 + 0.25 + 0.35 = 1.0
		},
		{
			name:       "all zeros",
			likelihood: 0.0,
			impact:     0.0,
			evidence:   0.0,
			wantTotal:  0.0,
		},
		{
			name:       "only likelihood",
			likelihood: 0.8,
			impact:     0.0,
			evidence:   0.0,
			wantTotal:  0.32, // 0.8 * 0.40
		},
		{
			name:       "only impact",
			likelihood: 0.0,
			impact:     0.6,
			evidence:   0.0,
			wantTotal:  0.15, // 0.6 * 0.25
		},
		{
			name:       "only evidence",
			likelihood: 0.0,
			impact:     0.0,
			evidence:   0.5,
			wantTotal:  0.175, // 0.5 * 0.35
		},
		{
			name:       "mixed values",
			likelihood: 0.9,
			impact:     0.7,
			evidence:   0.8,
			wantTotal:  0.815, // 0.9*0.40 + 0.7*0.25 + 0.8*0.35 = 0.36 + 0.175 + 0.28
		},
		{
			name:       "half scores",
			likelihood: 0.5,
			impact:     0.5,
			evidence:   0.5,
			wantTotal:  0.5, // 0.5*(0.40+0.25+0.35) = 0.5
		},
	}

	const epsilon = 1e-9
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Score{
				Likelihood: tt.likelihood,
				Impact:     tt.impact,
				Evidence:   tt.evidence,
			}
			CalculateTotal(s)
			if math.Abs(s.Total-tt.wantTotal) > epsilon {
				t.Errorf("CalculateTotal() = %f, want %f (L=%f, I=%f, E=%f)",
					s.Total, tt.wantTotal, tt.likelihood, tt.impact, tt.evidence)
			}
		})
	}
}

func TestDetectTraps(t *testing.T) {
	trapTexts := []struct {
		name string
		text string
	}{
		{"restart the pod", "We should restart the pod to fix the issue"},
		{"restart the node", "Restart the node and see if it comes back"},
		{"delete and recreate", "Delete and recreate the deployment"},
		{"increase the limits", "Try to increase resource limits for the container"},
		{"scale up", "We need to scale up the replica count"},
		{"disable the webhook", "Disable the webhook temporarily to unblock"},
		{"force delete", "Force delete the stuck namespace"},
	}

	for _, tt := range trapTexts {
		t.Run(tt.name, func(t *testing.T) {
			h := &Hypothesis{
				Text:      tt.text,
				Rationale: "",
			}
			if !DetectTraps(h) {
				t.Errorf("DetectTraps() = false for %q, want true", tt.text)
			}
			if h.Score.Trap == "" {
				t.Error("DetectTraps() did not set Score.Trap reason")
			}
		})
	}

	// Verify trap detection works on Rationale field too.
	t.Run("trap in rationale", func(t *testing.T) {
		h := &Hypothesis{
			Text:      "Something generic",
			Rationale: "The fix is to restart the pod",
		}
		if !DetectTraps(h) {
			t.Error("DetectTraps() did not detect trap pattern in Rationale")
		}
	})

	// Verify case insensitivity.
	t.Run("case insensitive", func(t *testing.T) {
		h := &Hypothesis{
			Text: "RESTART THE POD immediately",
		}
		if !DetectTraps(h) {
			t.Error("DetectTraps() did not detect trap with uppercase text")
		}
	})
}

func TestDetectTrapsClean(t *testing.T) {
	cleanHypotheses := []struct {
		name      string
		text      string
		rationale string
	}{
		{
			name:      "etcd disk latency",
			text:      "etcd WAL fsync latency exceeds 100ms due to noisy-neighbor I/O contention",
			rationale: "The fdatasync p99 is spiking which causes leader election instability",
		},
		{
			name:      "certificate expiry",
			text:      "Intermediate CA certificate expired causing API server TLS handshake failures",
			rationale: "The certificate was issued 2 years ago and was not auto-rotated",
		},
		{
			name:      "network policy blocking",
			text:      "NetworkPolicy in the namespace blocks egress to the image registry",
			rationale: "Default deny policy was applied without exemptions for registry traffic",
		},
		{
			name:      "resource quota",
			text:      "ResourceQuota in the namespace prevents new pod creation",
			rationale: "The namespace quota for CPU requests has been exhausted by existing deployments",
		},
		{
			name:      "OVN flow misconfiguration",
			text:      "OVN logical switch port has stale MAC binding causing packet drops",
			rationale: "After a node reboot the OVN northbound database was not reconciled",
		},
	}

	for _, tt := range cleanHypotheses {
		t.Run(tt.name, func(t *testing.T) {
			h := &Hypothesis{
				Text:      tt.text,
				Rationale: tt.rationale,
			}
			if DetectTraps(h) {
				t.Errorf("DetectTraps() flagged clean hypothesis %q as trap: %s",
					tt.text, h.Score.Trap)
			}
		})
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.FrameCount != 5 {
		t.Errorf("DefaultOptions().FrameCount = %d, want 5", opts.FrameCount)
	}
	if opts.TopK != 3 {
		t.Errorf("DefaultOptions().TopK = %d, want 3", opts.TopK)
	}
	if opts.Depth != "standard" {
		t.Errorf("DefaultOptions().Depth = %q, want %q", opts.Depth, "standard")
	}
	if opts.Concurrency != 4 {
		t.Errorf("DefaultOptions().Concurrency = %d, want 4", opts.Concurrency)
	}
}

func TestDetectTrapsAll(t *testing.T) {
	hypotheses := []Hypothesis{
		{ID: "h1", Text: "restart the pod to fix OOM", Rationale: "quick fix"},
		{ID: "h2", Text: "etcd leader election is unstable", Rationale: "disk latency is too high"},
		{ID: "h3", Text: "force delete the stuck PV", Rationale: "unblock the pipeline"},
		{ID: "h4", Text: "CSI driver socket is missing", Rationale: "node was reimaged"},
	}

	traps := DetectTrapsAll(hypotheses)

	if got := len(traps); got != 2 {
		t.Fatalf("DetectTrapsAll() returned %d traps, want 2", got)
	}

	trapIDs := make(map[string]bool)
	for _, tr := range traps {
		trapIDs[tr.ID] = true
	}
	if !trapIDs["h1"] {
		t.Error("DetectTrapsAll() did not flag h1 (restart the pod)")
	}
	if !trapIDs["h3"] {
		t.Error("DetectTrapsAll() did not flag h3 (force delete)")
	}
}
