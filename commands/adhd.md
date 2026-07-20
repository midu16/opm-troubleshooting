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

**Integration**: Before divergence, runs `inspect-bundle` to get current metadata state. Uses output as seed for divergent thinking about why labels are missing.

### Example 2: Channel migration produces unexpected behavior
```bash
/opm-troubleshooting:adhd \
  --catalog quay.io/prega/prega-operator-index:v4.22-latest \
  --package odf-operator \
  --channel stable-4.21 \
  "Migrating from stable-4.21 to stable-4.22 causes subscription failure - what could be the root cause?"
```

**Integration**: Uses `resolve-channel` to verify both channels exist, then runs `inspect-bundle` on both heads to seed divergence with version/commit differences.

### Example 3: Pattern in batch validation failures
```bash
/opm-troubleshooting:adhd \
  --list operators-to-audit.txt \
  "Multiple operators showing PARTIAL status - missing commit metadata. Is this a systematic issue or per-operator?"
```

**Integration**: Runs `batch-validate` first to get full picture, then diverges on whether the pattern indicates catalog build process issues vs individual operator Dockerfile problems.

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
