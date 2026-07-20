---
description: Divergent ideation for OLM troubleshooting using parallel cognitive frames with existing tool integration
argument-hint: "--catalog <image> --package <name> [--channel <name>] 'problem description'"
tags: ["ideation", "troubleshooting", "brainstorming"]
---

## Name
opm-troubleshooting:adhd

## Synopsis
```
/opm-troubleshooting:adhd --catalog <catalog-image> --package <name> [--channel <channel>] 'describe your OLM problem'
```

## Attribution
Divergent ideation framework inspired by [ADHD-Agent](https://github.com/UditAkhourii/adhd) (MIT License). Adapted with OLM-specific cognitive frames and integration with existing OPM tools.

## Description
The `opm-troubleshooting:adhd` command applies **divergent ideation** to OLM troubleshooting by spawning parallel diagnostic approaches across multiple cognitive frames. Unlike single-path debugging, this explores 30+ potential root causes or solutions simultaneously using isolated reasoning branches that can't anchor each other.

This is especially valuable when:
- A bundle fails with an ambiguous error (PARTIAL status, missing metadata)
- Channel migration produces unexpected behavior
- Batch validation reveals patterns but not root causes
- You need creative solutions for catalog quality issues

**Integration with existing tools:**
- Uses `inspect-bundle` output as seed data for divergence
- Leverages `resolve-channel` to understand upgrade paths
- Incorporates `batch-validate` results when analyzing multiple operators
- References agent workflows (Bundle Metadata Analyzer, Channel Migration Assistant) for deep-dive follow-up

## Implementation

The command executes a two-phase divergent ideation loop:

### Phase 1 — Diverge (parallel, no evaluation)
Spawns 5 isolated reasoning branches, each using a different cognitive frame:

1. **Hardware/Firmware Frame**: Think in latency, memory layout, image pull constraints. What does the OCI registry topology or network timeout budget reveal?
2. **Regulator/Audit Frame**: Audit for compliance gaps. What must be provable, traceable, or refusable here? Which OpenShift standards are violated?
3. **Competitor/Attacker Frame**: How would a hostile party exploit this failure mode? Invert sabotage into defensive fixes.
4. **Biology/Evolution Frame**: Transplant immune system or cell signaling mechanisms. How does an OLM "immune response" to bad bundles work?
5. **10-Year-Old Naive Frame**: Describe the problem without any OLM knowledge. What obvious solution is being overlooked because it's too simple?

Each branch generates 6 distinct diagnostic hypotheses or fix strategies, pushing past the first three obvious answers (which are usually wrong for non-obvious failures).

### Phase 2 — Focus (evaluate and converge)
After all branches return:

1. **Score** each idea on: novelty (distance from default troubleshooting), viability (could it actually ship/fix), fit (does it address the stated problem)
2. **Cluster** into 3-6 groups by underlying angle, not surface keywords
3. **Deepen top 3**: For each promising branch, sketch how to verify it using existing tools (inspect-bundle for metadata checks, resolve-channel for path validation), name load-bearing risks, and suggest first concrete diagnostic step

## Return Value
Structured output with:
- **Brief summary** of the problem and reframing used
- **Wide set** of ideas grouped by cluster, each scored `[N7 V8 F9]`
- **Converge**: 2-4 idea shortlist with explicit non-obvious picks marked ★
- **Focus**: 3 deepened branches with verification steps using OPM tools
- **Provocation**: One wildcard question opening new diagnostic direction

## Examples

### Example 1: Troubleshoot PARTIAL bundle status
```bash
/opm-troubleshooting:adhd \
  --catalog quay.io/prega/prega-operator-index:v4.22-latest \
  --package kubernetes-nmstate-operator \
  "Bundle shows PARTIAL status - missing commit SHA and repository URL in image labels"
```

**Integration**: 
1. Runs `inspect-bundle --json` to get current metadata state (seed data)
2. Spawns 5 branches: Registry Auditor, Bundle Forensics, Channel Historian, Catalog Architect, Naive User
3. Each branch generates 6 hypotheses about why labels are missing
4. Converges to top 3 with verification steps using `inspect-bundle`

**Expected Output Structure**:
- Wide set grouped by cluster (Label Inheritance, Build Process, Channel History)
- ★ Non-obvious pick: "Base image lacks commit labels → bundle inherits empty values"
- Focus: Sketch + risk + first step (e.g., "Check base image with `skopeo inspect`")

### Example 2: Channel migration produces unexpected behavior
```bash
/opm-troubleshooting:adhd \
  --catalog quay.io/prega/prega-operator-index:v4.22-latest \
  --package odf-operator \
  --channel stable-4.21 \
  "Migrating from stable-4.21 to stable-4.22 causes subscription failure - what could be the root cause?"
```

**Integration**: 
1. Uses `resolve-channel` to verify both channels exist (seed data)
2. Runs `inspect-bundle` on both channel heads for version/commit comparison
3. Spawns 5 branches: Channel Historian, Subscription Simulator, Bundle Forensics, Attacker Frame, OLM Internals
4. Diverges on breaking changes, dependency shifts, label format changes
5. Converges with migration verification steps

**Key Diagnostic Questions**:
- Which version introduced the breaking change?
- Are there missing `olm.package` declarations in target channel?
- What InstallPlan would be generated and where does it fail?

### Example 3: Pattern in batch validation failures
```bash
/opm-troubleshooting:adhd \
  --list operators-to-audit.txt \
  "Multiple operators showing PARTIAL status - missing commit metadata. Is this a systematic issue or per-operator?"
```

**Integration**: 
1. Runs `batch-validate` to get full picture (seed data)
2. Spawns 5 branches: Catalog Architect, Registry Auditor, Bundle Forensics, Regulator Frame, Biology/Evolution
3. Diverges on whether pattern indicates systematic issue vs per-operator problems
4. Converges with catalog build process improvements or per-operator Dockerfile fixes

**Output Highlights**:
- Cluster analysis: "Systematic (70%)" vs "Per-operator (30%)"
- ★ Non-obvious pick: "Catalog build pipeline drops labels in final stage"
- Focus: Verification steps using `batch-validate` on subset, then deep-dive on affected operators

## Advanced Integration Patterns

### Pattern A: Chaining with Existing Agents
```bash
# Step 1: Use ADHD to explore diagnostic approaches
/opm-troubleshooting:adhd \
  --catalog ... --package ... "PARTIAL status issue"

# Step 2: Follow up with targeted agent for deep investigation
/opm-troubleshooting:analyze-metadata \
  --catalog ... --package ... 
```

### Pattern B: Iterative Refinement
```bash
# First pass: Broad exploration (5 frames)
OPM_AHDH_FRAMES="registry-auditor,bundle-forensics,channel-historian" \
/opm-troubleshooting:adhd --catalog ... --package ... "problem"

# Second pass: Deep dive on promising cluster (3 frames)
OPM_AHDH_FRAMES="bundle-forensics,catalog-architect" \
OPM_AHDH_TOP_K=5 \
/opm-troubleshooting:adhd --catalog ... --package ... "problem"
```

### Pattern C: Batch Pattern Detection
```bash
# Step 1: Identify patterns with batch-validate
/opm-troubleshooting:batch-validate --catalog ... --list operators.txt > results.txt

# Step 2: Use ADHD to interpret the pattern
grep PARTIAL results.txt | awk '{print $1}' > partial-operators.txt
cat partial-operators.txt | xargs -I {} \
  /opm-troubleshooting:adhd --catalog ... --package {} "missing commit metadata"
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OPM_AHDH_FRAMES` | Auto-selected | Override which cognitive frames to use (comma-separated) |
| `OPM_AHDH_TOP_K` | 3 | Number of ideas to deepen/focus on |
| `OPM_AHDH_CONCURRENCY` | 4 | Maximum parallel LLM calls during divergence |

### Frame Selection Strategy

By default, the command selects frames based on problem context:

- **Code-shaped problems** (image pulls, metadata extraction): 4 code/design frames + 1 wild frame
- **Design-shaped problems** (channel topology, migration paths): 3 design frames + 2 code frames + 1 wild frame
- **Open-ended strategy**: Mix from all tags, ensure at least one wildcard

Override with `OPM_AHDH_FRAMES`:
```bash
export OPM_AHDH_FRAMES="registry-auditor,bundle-forensics,channel-historian"
/opm-troubleshooting:adhd --catalog ... --package ... "problem description"
```

## Arguments

- `--catalog` (required): OLM catalog index image reference for context
- `--package` (required): Operator package name being diagnosed
- `--channel` (optional): Specific channel to focus divergence on
- `--list` (optional): File with operator list for pattern-level divergence
- Problem description: Natural language description of the issue or question

## Environment Variables

- `DOCKER_CONFIG`: Path to registry credentials (same as other commands)
- `OPM_AHDH_FRAMES` (optional): Override which cognitive frames to use (comma-separated, e.g., "hardware,regulator,biology")

## Notes

- This command is **not** a replacement for targeted debugging - it's for when the obvious path isn't working
- For simple fixes (missing channel name), use `resolve-channel` directly instead
- The divergence phase does NOT call OPM tools - it's pure reasoning over the problem space
- The focus phase RECOMMENDS which existing tools to run next based on promising branches
- Cost: ~5 parallel agent calls + 1 converge step = ~6x slower than direct debugging, but catches blind spots

## See Also

- `opm-troubleshooting:inspect-bundle` - Direct metadata inspection (use for targeted checks)
- `opm-troubleshooting:resolve-channel` - Channel resolution (use for path validation)
- `opm-troubleshooting:batch-validate` - Batch validation (use for pattern detection)
- Agents/AGENTS.md - Deep-dive agent workflows for follow-up investigation
