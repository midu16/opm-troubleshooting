package adhd

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/midu16/opm-troubleshooting/internal/claudeapi"
	"github.com/midu16/opm-troubleshooting/internal/datasource"
)

// Engine orchestrates the ADHD diverge-converge diagnostic loop.
type Engine struct {
	claude *claudeapi.Client
}

// NewEngine creates an ADHD diagnostic engine backed by the given Claude client.
func NewEngine(claude *claudeapi.Client) *Engine {
	return &Engine{claude: claude}
}

// Diagnose runs the full ADHD diagnostic cycle: diverge -> score -> cluster -> deepen.
func (e *Engine) Diagnose(ctx context.Context, problem string, symptoms []string, clusterData *ClusterSnapshot, opts DiagnosisOptions) (*DiagnosisResult, error) {
	if opts.FrameCount <= 0 {
		opts.FrameCount = DefaultOptions().FrameCount
	}
	if opts.TopK <= 0 {
		opts.TopK = DefaultOptions().TopK
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = DefaultOptions().Concurrency
	}
	if opts.Depth == "" {
		opts.Depth = DefaultOptions().Depth
	}

	// --- Phase 1: Diverge ---
	frames := SelectFrames(nil, opts.FrameCount)
	branches, err := e.diverge(ctx, problem, symptoms, clusterData, frames, opts.Concurrency)
	if err != nil {
		return nil, fmt.Errorf("diverge phase: %w", err)
	}

	// Collect all hypotheses across branches.
	var allHypotheses []Hypothesis
	for _, b := range branches {
		allHypotheses = append(allHypotheses, b.Hypotheses...)
	}
	if len(allHypotheses) == 0 {
		return &DiagnosisResult{Problem: problem, Branches: branches}, nil
	}

	// --- Phase 2: Score ---
	if err := e.score(ctx, problem, allHypotheses); err != nil {
		return nil, fmt.Errorf("score phase: %w", err)
	}

	// Detect traps.
	traps := DetectTrapsAll(allHypotheses)

	// Sort by total score descending.
	sort.Slice(allHypotheses, func(i, j int) bool {
		return allHypotheses[i].Score.Total > allHypotheses[j].Score.Total
	})

	// Build shortlist.
	shortlist := allHypotheses
	if len(shortlist) > opts.TopK {
		shortlist = shortlist[:opts.TopK]
	}

	// Pick non-obvious: highest-scored hypothesis from a "wild" frame not already in shortlist.
	var nonObvious *Hypothesis
	shortlistIDs := make(map[string]bool, len(shortlist))
	for _, h := range shortlist {
		shortlistIDs[h.ID] = true
	}
	for i, h := range allHypotheses {
		if shortlistIDs[h.ID] {
			continue
		}
		f := findFrame(frames, h.FrameID)
		if f != nil && frameHasTag(*f, "wild") {
			nonObvious = &allHypotheses[i]
			break
		}
	}

	// --- Phase 3: Cluster ---
	clusters, err := e.cluster(ctx, problem, allHypotheses)
	if err != nil {
		// Clustering is best-effort; continue without it.
		clusters = nil
	}

	// --- Phase 4: Deepen ---
	var deepened []DeepenedHypothesis
	if opts.Depth != "quick" {
		deepened, err = e.deepen(ctx, problem, shortlist, clusterData)
		if err != nil {
			// Deepening is best-effort; continue without it.
			deepened = nil
		}
	}

	// --- Provocation ---
	provocation, _ := e.provoke(ctx, problem, shortlist)

	return &DiagnosisResult{
		Problem:     problem,
		Branches:    branches,
		Clusters:    clusters,
		Shortlist:   shortlist,
		NonObvious:  nonObvious,
		Traps:       traps,
		Deepened:    deepened,
		Provocation: provocation,
	}, nil
}

// ---------------------------------------------------------------------------
// Phase 1: Diverge
// ---------------------------------------------------------------------------

// diverge spawns parallel analysis frames, each getting an isolated prompt.
func (e *Engine) diverge(ctx context.Context, problem string, symptoms []string, snap *ClusterSnapshot, frames []Frame, concurrency int) ([]Branch, error) {
	sem := make(chan struct{}, concurrency)

	type result struct {
		branch Branch
		err    error
		index  int
	}

	results := make([]result, len(frames))
	var wg sync.WaitGroup

	for i, frame := range frames {
		wg.Add(1)
		go func(idx int, f Frame) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			prompt := buildDivergePrompt(problem, symptoms, snap, f)
			systemPrompt := "You are an expert OpenShift/Kubernetes diagnostic analyst operating " +
				"from a specific vantage point. Generate exactly 3-5 hypotheses. Respond ONLY with " +
				"a JSON array of objects, each with fields: \"id\" (string, unique), \"text\" (string, " +
				"one-sentence hypothesis), \"rationale\" (string, 2-3 sentences explaining why), " +
				"\"evidence\" (array of strings, observable indicators). No markdown, no commentary."

			raw, err := e.claude.Complete(ctx, systemPrompt, prompt, 4096)
			if err != nil {
				results[idx] = result{err: fmt.Errorf("frame %s: %w", f.ID, err), index: idx}
				return
			}

			hypotheses := parseDivergeResponse(raw, f.ID)
			results[idx] = result{
				branch: Branch{
					FrameID:    f.ID,
					FrameName:  f.Name,
					Hypotheses: hypotheses,
				},
				index: idx,
			}
		}(i, frame)
	}

	wg.Wait()

	var branches []Branch
	var errs []string
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err.Error())
			continue
		}
		branches = append(branches, r.branch)
	}

	if len(branches) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("all frames failed: %s", strings.Join(errs, "; "))
	}

	return branches, nil
}

// buildDivergePrompt constructs the user prompt for a single frame.
func buildDivergePrompt(problem string, symptoms []string, snap *ClusterSnapshot, frame Frame) string {
	var sb strings.Builder

	sb.WriteString("## Your Vantage Point\n")
	sb.WriteString(frame.VantagePrompt)
	sb.WriteString("\n\n## Problem Statement\n")
	sb.WriteString(problem)
	sb.WriteString("\n\n## Observed Symptoms\n")
	for _, s := range symptoms {
		sb.WriteString("- ")
		sb.WriteString(s)
		sb.WriteString("\n")
	}

	if snap != nil {
		sb.WriteString("\n## Cluster State Summary\n")
		sb.WriteString(formatSnapshot(snap))
	}

	sb.WriteString("\n## Task\n")
	sb.WriteString("Generate 3-5 hypotheses that explain the problem from your vantage point. ")
	sb.WriteString("Each hypothesis must be specific and testable. Rank them by likelihood.")

	return sb.String()
}

// formatSnapshot renders a ClusterSnapshot as a human-readable summary for prompting.
func formatSnapshot(snap *ClusterSnapshot) string {
	var sb strings.Builder

	if snap.ClusterVersion != "" {
		fmt.Fprintf(&sb, "- Cluster version: %s\n", snap.ClusterVersion)
	}
	fmt.Fprintf(&sb, "- Nodes: %d total, %d not ready\n", snap.NodeCount, snap.NotReadyNodes)
	if len(snap.DegradedOperators) > 0 {
		fmt.Fprintf(&sb, "- Degraded operators: %s\n", strings.Join(snap.DegradedOperators, ", "))
	}
	if len(snap.FailedPods) > 0 {
		display := snap.FailedPods
		if len(display) > 10 {
			display = append(display[:10], fmt.Sprintf("... and %d more", len(snap.FailedPods)-10))
		}
		fmt.Fprintf(&sb, "- Failed pods: %s\n", strings.Join(display, ", "))
	}
	if len(snap.WarningEvents) > 0 {
		display := snap.WarningEvents
		if len(display) > 10 {
			display = append(display[:10], fmt.Sprintf("... and %d more", len(snap.WarningEvents)-10))
		}
		fmt.Fprintf(&sb, "- Warning events: %s\n", strings.Join(display, "; "))
	}
	if snap.MCPStatus != "" {
		fmt.Fprintf(&sb, "- MCP status: %s\n", snap.MCPStatus)
	}
	if snap.NetworkType != "" {
		fmt.Fprintf(&sb, "- Network type: %s\n", snap.NetworkType)
	}
	if len(snap.StorageIssues) > 0 {
		fmt.Fprintf(&sb, "- Storage issues: %s\n", strings.Join(snap.StorageIssues, "; "))
	}

	return sb.String()
}

// parseDivergeResponse extracts hypotheses from a Claude JSON response.
func parseDivergeResponse(raw string, frameID string) []Hypothesis {
	// Strip markdown code fences if present.
	cleaned := strings.TrimSpace(raw)
	if strings.HasPrefix(cleaned, "```") {
		if idx := strings.Index(cleaned[3:], "\n"); idx != -1 {
			cleaned = cleaned[3+idx+1:]
		}
		if idx := strings.LastIndex(cleaned, "```"); idx != -1 {
			cleaned = cleaned[:idx]
		}
		cleaned = strings.TrimSpace(cleaned)
	}

	// Try parsing as JSON array.
	var items []struct {
		ID        string   `json:"id"`
		Text      string   `json:"text"`
		Rationale string   `json:"rationale"`
		Evidence  []string `json:"evidence"`
	}

	if err := json.Unmarshal([]byte(cleaned), &items); err != nil {
		// Fallback: treat the entire response as a single hypothesis.
		return []Hypothesis{
			{
				ID:        fmt.Sprintf("%s-fallback-1", frameID),
				FrameID:   frameID,
				Text:      truncate(cleaned, 200),
				Rationale: cleaned,
				Depth:     0,
			},
		}
	}

	hypotheses := make([]Hypothesis, 0, len(items))
	for i, item := range items {
		id := item.ID
		if id == "" {
			id = fmt.Sprintf("%s-%d", frameID, i+1)
		}
		hypotheses = append(hypotheses, Hypothesis{
			ID:        id,
			FrameID:   frameID,
			Text:      item.Text,
			Rationale: item.Rationale,
			Evidence:  item.Evidence,
			Depth:     0,
		})
	}

	return hypotheses
}

// ---------------------------------------------------------------------------
// Phase 2: Score
// ---------------------------------------------------------------------------

// score asks Claude to critically score each hypothesis.
func (e *Engine) score(ctx context.Context, problem string, hypotheses []Hypothesis) error {
	// Build a JSON representation of hypotheses for the scoring prompt.
	type scoringInput struct {
		ID        string `json:"id"`
		Text      string `json:"text"`
		Rationale string `json:"rationale"`
	}

	inputs := make([]scoringInput, len(hypotheses))
	for i, h := range hypotheses {
		inputs[i] = scoringInput{ID: h.ID, Text: h.Text, Rationale: h.Rationale}
	}

	inputJSON, err := json.Marshal(inputs)
	if err != nil {
		return fmt.Errorf("marshal scoring input: %w", err)
	}

	systemPrompt := "You are a critical evaluator of OpenShift troubleshooting hypotheses. " +
		"Score each hypothesis on three dimensions (0.0 to 1.0): " +
		"\"likelihood\" (probability this is the actual root cause), " +
		"\"impact\" (severity if this hypothesis is correct), " +
		"\"evidence\" (how well the available evidence supports this hypothesis). " +
		"Respond ONLY with a JSON array of objects, each with fields: " +
		"\"id\" (matching the input), \"likelihood\" (float), \"impact\" (float), \"evidence\" (float). " +
		"No markdown, no commentary."

	userPrompt := fmt.Sprintf("## Problem\n%s\n\n## Hypotheses to Score\n%s", problem, string(inputJSON))

	raw, err := e.claude.Complete(ctx, systemPrompt, userPrompt, 4096)
	if err != nil {
		return fmt.Errorf("scoring API call: %w", err)
	}

	// Parse the scores.
	type scoreResult struct {
		ID         string  `json:"id"`
		Likelihood float64 `json:"likelihood"`
		Impact     float64 `json:"impact"`
		Evidence   float64 `json:"evidence"`
	}

	cleaned := stripCodeFences(raw)
	var scores []scoreResult
	if err := json.Unmarshal([]byte(cleaned), &scores); err != nil {
		// If parsing fails, assign default scores so the pipeline continues.
		for i := range hypotheses {
			hypotheses[i].Score = Score{Likelihood: 0.5, Impact: 0.5, Evidence: 0.5}
			CalculateTotal(&hypotheses[i].Score)
		}
		return nil
	}

	// Map scores back to hypotheses by ID.
	scoreMap := make(map[string]scoreResult, len(scores))
	for _, s := range scores {
		scoreMap[s.ID] = s
	}

	for i := range hypotheses {
		if s, ok := scoreMap[hypotheses[i].ID]; ok {
			hypotheses[i].Score = Score{
				Likelihood: clampFloat(s.Likelihood, 0, 1),
				Impact:     clampFloat(s.Impact, 0, 1),
				Evidence:   clampFloat(s.Evidence, 0, 1),
			}
		} else {
			hypotheses[i].Score = Score{Likelihood: 0.5, Impact: 0.5, Evidence: 0.5}
		}
		CalculateTotal(&hypotheses[i].Score)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Phase 3: Cluster
// ---------------------------------------------------------------------------

// cluster groups hypotheses by shared failure mechanism.
func (e *Engine) cluster(ctx context.Context, problem string, hypotheses []Hypothesis) ([]Cluster, error) {
	type clusterInput struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}

	inputs := make([]clusterInput, len(hypotheses))
	for i, h := range hypotheses {
		inputs[i] = clusterInput{ID: h.ID, Text: h.Text}
	}

	inputJSON, err := json.Marshal(inputs)
	if err != nil {
		return nil, fmt.Errorf("marshal cluster input: %w", err)
	}

	systemPrompt := "You group OpenShift troubleshooting hypotheses by their underlying failure " +
		"mechanism. Two hypotheses share a mechanism if fixing one would likely fix the other. " +
		"Respond ONLY with a JSON array of objects, each with fields: " +
		"\"label\" (short name for the cluster), \"mechanism\" (one-sentence description of the " +
		"shared failure mechanism), \"hypothesis_ids\" (array of hypothesis ID strings). " +
		"Every hypothesis ID must appear in exactly one cluster. No markdown, no commentary."

	userPrompt := fmt.Sprintf("## Problem\n%s\n\n## Hypotheses\n%s", problem, string(inputJSON))

	raw, err := e.claude.Complete(ctx, systemPrompt, userPrompt, 4096)
	if err != nil {
		return nil, fmt.Errorf("clustering API call: %w", err)
	}

	cleaned := stripCodeFences(raw)
	var clusters []Cluster
	if err := json.Unmarshal([]byte(cleaned), &clusters); err != nil {
		return nil, fmt.Errorf("parse cluster response: %w", err)
	}

	return clusters, nil
}

// ---------------------------------------------------------------------------
// Phase 4: Deepen
// ---------------------------------------------------------------------------

// deepen generates investigation guidance for top hypotheses.
func (e *Engine) deepen(ctx context.Context, problem string, shortlist []Hypothesis, snap *ClusterSnapshot) ([]DeepenedHypothesis, error) {
	deepened := make([]DeepenedHypothesis, 0, len(shortlist))

	for _, h := range shortlist {
		d, err := e.deepenOne(ctx, problem, h, snap)
		if err != nil {
			// Skip individual failures.
			continue
		}
		deepened = append(deepened, *d)
	}

	return deepened, nil
}

// deepenOne generates investigation guidance for a single hypothesis.
func (e *Engine) deepenOne(ctx context.Context, problem string, h Hypothesis, snap *ClusterSnapshot) (*DeepenedHypothesis, error) {
	systemPrompt := "You are an OpenShift diagnostic specialist. Given a hypothesis about a cluster " +
		"problem, produce a detailed investigation plan. Respond ONLY with a JSON object containing: " +
		"\"sketch\" (string: 2-3 paragraph investigation plan with specific oc/kubectl commands), " +
		"\"load_bearing_risk\" (string: what is the single riskiest assumption in this hypothesis), " +
		"\"first_step\" (string: the ONE command or check to run first), " +
		"\"child_hypotheses\" (array of objects with \"id\", \"text\", \"rationale\" fields: " +
		"sub-hypotheses that would emerge if this hypothesis is confirmed). " +
		"No markdown, no commentary."

	var snapText string
	if snap != nil {
		snapText = "\n\n## Cluster State\n" + formatSnapshot(snap)
	}

	userPrompt := fmt.Sprintf("## Problem\n%s%s\n\n## Hypothesis to Deepen\n**%s**\n\nRationale: %s\n\nScore: likelihood=%.2f impact=%.2f evidence=%.2f",
		problem, snapText, h.Text, h.Rationale, h.Score.Likelihood, h.Score.Impact, h.Score.Evidence)

	raw, err := e.claude.Complete(ctx, systemPrompt, userPrompt, 4096)
	if err != nil {
		return nil, fmt.Errorf("deepen API call: %w", err)
	}

	cleaned := stripCodeFences(raw)

	var parsed struct {
		Sketch          string `json:"sketch"`
		LoadBearingRisk string `json:"load_bearing_risk"`
		FirstStep       string `json:"first_step"`
		ChildHypotheses []struct {
			ID        string `json:"id"`
			Text      string `json:"text"`
			Rationale string `json:"rationale"`
		} `json:"child_hypotheses"`
	}

	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		// Fallback: use the raw text as the sketch.
		return &DeepenedHypothesis{
			HypothesisID: h.ID,
			Sketch:       cleaned,
		}, nil
	}

	result := &DeepenedHypothesis{
		HypothesisID:    h.ID,
		Sketch:          parsed.Sketch,
		LoadBearingRisk: parsed.LoadBearingRisk,
		FirstStep:       parsed.FirstStep,
	}

	for _, ch := range parsed.ChildHypotheses {
		result.ChildHypotheses = append(result.ChildHypotheses, Hypothesis{
			ID:        ch.ID,
			FrameID:   h.FrameID,
			Text:      ch.Text,
			Rationale: ch.Rationale,
			Depth:     h.Depth + 1,
			ParentID:  h.ID,
		})
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Provocation
// ---------------------------------------------------------------------------

// provoke generates a single provocative question to challenge assumptions.
func (e *Engine) provoke(ctx context.Context, problem string, shortlist []Hypothesis) (string, error) {
	var hypothesisSummary strings.Builder
	for _, h := range shortlist {
		fmt.Fprintf(&hypothesisSummary, "- %s (score: %.2f)\n", h.Text, h.Score.Total)
	}

	systemPrompt := "You challenge OpenShift troubleshooting teams. Given the current top hypotheses, " +
		"generate ONE provocative question that exposes a blind spot or unstated assumption the team " +
		"may be making. The question should make them uncomfortable but productive. Respond with " +
		"just the question, nothing else."

	userPrompt := fmt.Sprintf("## Problem\n%s\n\n## Current Top Hypotheses\n%s",
		problem, hypothesisSummary.String())

	return e.claude.Complete(ctx, systemPrompt, userPrompt, 512)
}

// ---------------------------------------------------------------------------
// BuildClusterSnapshot
// ---------------------------------------------------------------------------

// BuildClusterSnapshot collects relevant data from a ClusterDataSource and
// returns a serializable snapshot suitable for prompt construction.
func BuildClusterSnapshot(src datasource.ClusterDataSource) (*ClusterSnapshot, error) {
	snap := &ClusterSnapshot{}

	// Cluster version.
	if cv, err := src.GetClusterVersion(); err == nil && cv != nil {
		snap.ClusterVersion = cv.Version
	}

	// Nodes.
	if nodes, err := src.GetNodes(); err == nil {
		snap.NodeCount = len(nodes)
		for _, n := range nodes {
			if !n.Ready {
				snap.NotReadyNodes++
			}
		}
	}

	// Degraded cluster operators.
	if cos, err := src.GetClusterOperators(); err == nil {
		for _, co := range cos {
			if co.Degraded || !co.Available {
				snap.DegradedOperators = append(snap.DegradedOperators, co.Name)
			}
		}
	}

	// Failed pods across all namespaces.
	if namespaces, err := src.GetNamespaces(); err == nil {
		for _, ns := range namespaces {
			if pods, err := src.GetPods(ns); err == nil {
				for _, p := range pods {
					if p.Phase == "Failed" || p.Phase == "CrashLoopBackOff" || (!p.Ready && p.Phase == "Running") || p.WaitingReason != "" {
						snap.FailedPods = append(snap.FailedPods, fmt.Sprintf("%s/%s (%s)", ns, p.Name, p.Phase))
					}
				}
			}

			// Warning events.
			if events, err := src.GetEvents(ns); err == nil {
				for _, ev := range events {
					if ev.Type == "Warning" {
						snap.WarningEvents = append(snap.WarningEvents, fmt.Sprintf("[%s] %s: %s", ev.Object, ev.Reason, truncate(ev.Message, 120)))
					}
				}
			}
		}
	}

	// Limit warning events to avoid prompt bloat.
	if len(snap.WarningEvents) > 50 {
		snap.WarningEvents = snap.WarningEvents[:50]
	}

	// MCP status.
	if mcps, err := src.GetMachineConfigPools(); err == nil {
		var mcpParts []string
		for _, mcp := range mcps {
			status := "ok"
			if mcp.Degraded {
				status = "DEGRADED"
			} else if mcp.Updating {
				status = "updating"
			} else if mcp.Paused {
				status = "paused"
			}
			mcpParts = append(mcpParts, fmt.Sprintf("%s=%s (%d/%d ready)", mcp.Name, status, mcp.ReadyMachineCount, mcp.MachineCount))
		}
		snap.MCPStatus = strings.Join(mcpParts, ", ")
	}

	// Network type.
	if netCfg, err := src.GetNetworkConfig(); err == nil && netCfg != nil {
		snap.NetworkType = netCfg.NetworkType
	}

	// Storage issues.
	if pvs, err := src.GetPVs(); err == nil {
		for _, pv := range pvs {
			if pv.Phase == "Failed" || pv.Phase == "Released" {
				snap.StorageIssues = append(snap.StorageIssues, fmt.Sprintf("PV %s is %s", pv.Name, pv.Phase))
			}
		}
	}

	if namespaces, err := src.GetNamespaces(); err == nil {
		for _, ns := range namespaces {
			if pvcs, err := src.GetPVCs(ns); err == nil {
				for _, pvc := range pvcs {
					if pvc.Phase == "Pending" || pvc.Phase == "Lost" {
						snap.StorageIssues = append(snap.StorageIssues, fmt.Sprintf("PVC %s/%s is %s", ns, pvc.Name, pvc.Phase))
					}
				}
			}
		}
	}

	return snap, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// findFrame looks up a frame by ID in the given slice.
func findFrame(frames []Frame, id string) *Frame {
	for i := range frames {
		if frames[i].ID == id {
			return &frames[i]
		}
	}
	return nil
}

// stripCodeFences removes markdown code fences from a string.
func stripCodeFences(s string) string {
	cleaned := strings.TrimSpace(s)
	if strings.HasPrefix(cleaned, "```") {
		if idx := strings.Index(cleaned[3:], "\n"); idx != -1 {
			cleaned = cleaned[3+idx+1:]
		}
		if idx := strings.LastIndex(cleaned, "```"); idx != -1 {
			cleaned = cleaned[:idx]
		}
		cleaned = strings.TrimSpace(cleaned)
	}
	return cleaned
}

// truncate shortens a string to maxLen, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 4 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// clampFloat constrains v to [min, max].
func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
