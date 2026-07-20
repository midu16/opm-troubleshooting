# ADHD Integration: Divergent Ideation for OLM Troubleshooting

This document describes how the **ADHD (Adaptive Divergent Heuristic Discovery)** framework is integrated into `opm-troubleshooting` to expand diagnostic capabilities beyond single-path debugging.

## Overview

The `opm-troubleshooting:adhd` command applies divergent ideation to OLM troubleshooting by spawning parallel diagnostic approaches across multiple cognitive frames. Unlike traditional step-by-step debugging, this explores 30+ potential root causes or solutions simultaneously using isolated reasoning branches that cannot anchor each other.

**Inspired by**: [ADHD-Agent](https://github.com/UditAkhourii/adhd) (MIT License)  
**Attribution**: Divergent ideation framework adapted from upstream ADHD implementation with OLM-specific cognitive frames and tool integration.

## Architecture

### Two-Phase Loop

The command executes a strict two-phase loop with hard separation between phases:

```
┌─────────────────────────────────────────────────────────────┐
│  PHASE 1: DIVERGE (Parallel, No Evaluation)                │
├─────────────────────────────────────────────────────────────┤
│  Frame 1 ─┬──→ Idea 1.1                                    │
│           ├──→ Idea 1.2                                    │
│           ├──→ ...                                         │
│           └──→ Idea 1.N                                     │
│                                                             │
│  Frame 2 ─┬──→ Idea 2.1                                    │
│           ├──→ Idea 2.2                                    │
│           └──→ ...                                         │
│                                                             │
│  ... (5 frames total, zero shared context)                 │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│  PHASE 2: FOCUS (Critic on, Evaluate & Converge)           │
├─────────────────────────────────────────────────────────────┤
│  Score (novelty × viability × fit)                         │
│       ↓                                                     │
│  Cluster by underlying angle                               │
│       ↓                                                     │
│  Deepen top 3 with verification steps                      │
└─────────────────────────────────────────────────────────────┘
```

### Integration with Existing Tools

The ADHD command leverages existing OPM infrastructure:

| Phase | Tool Integration | Purpose |
|-------|------------------|---------|
| **Diverge** | None (pure reasoning) | Explore hypothesis space without tool bias |
| **Focus** | `inspect-bundle` | Verify metadata completeness for promising branches |
| **Focus** | `resolve-channel` | Validate upgrade paths before testing fixes |
| **Focus** | `batch-validate` | Pattern detection across multiple operators |

## OPM-Specific Cognitive Frames

Unlike generic ADHD, this integration uses frames tailored for OLM troubleshooting:

### 1. Registry Auditor Frame
**Vantage**: Think in OCI registry constraints, image pull failures, and digest mismatches.

> You are auditing an OCI registry for compliance and failure modes. What must be provable (image exists), traceable (digest matches manifest), or refusable (rate limits, auth failures) here? What does the registry topology tell you about why this bundle can't be pulled?

**Tags**: `code`, `design`  
**Best For**: Image pull failures, digest mismatches, authentication issues.

### 2. Channel Historian Frame
**Vantage**: Trace upgrade paths backward to find where things went wrong.

> You are reconstructing the upgrade history of an operator from channel head to initial install. What version introduced this breaking change? Which bundle in the chain first lacked commit metadata? Where did the migration path diverge from expected behavior?

**Tags**: `code`, `general`  
**Best For**: Channel migration failures, unexpected version behavior.

### 3. Bundle Forensics Frame
**Vantage**: Examine layer composition, manifest anomalies, and label inheritance.

> You are a forensic analyst examining a container image for tampering or misconfiguration. What layers were added/removed? Which labels inherited from base images are missing in the final bundle? Are there orphaned annotations or conflicting metadata between CSV and image labels?

**Tags**: `code`, `wild`  
**Best For**: Missing commit SHA, PARTIAL status, metadata inconsistencies.

### 4. Subscription Simulator Frame
**Vantage**: Simulate install plan execution to predict failure modes.

> You are the OLM operator simulating what happens when this subscription is applied. What InstallPlan would be generated? Which dependency resolution step fails first? Are there RBAC conflicts, CRD version mismatches, or namespace isolation issues that block installation?

**Tags**: `design`, `general`  
**Best For**: Subscription failures, dependency conflicts, RBAC issues.

### 5. Catalog Architect Frame
**Vantage**: Think in FBC structure, declarative config, and bundle topology.

> You are designing the File-Based Catalog from scratch. How would you structure the bundles to avoid this failure mode? Is the channel ordering correct? Are there circular references or missing `olm.package` declarations that break resolution?

**Tags**: `code`, `design`  
**Best For**: Catalog build failures, FBC validation errors, channel topology issues.

### 6. Regulator/Audit Frame (Generic)
**Vantage**: What must be provable, traceable, or refusable here?

> You are auditing systems for compliance and failure modes. What OpenShift standards are violated? Which metadata fields are required by OLM spec but missing? What would prevent this operator from passing a certification audit?

**Tags**: `design`, `general`  
**Best For**: Compliance gaps, missing required fields.

### 7. Attacker/Competitor Frame (Generic)
**Vantage**: How would a hostile party exploit this failure mode? Invert sabotage into defensive fixes.

> You are a malicious actor trying to sabotage operator installation. What bundle manipulation would cause the worst failure? Now invert: how do we detect and prevent this? What validation checks should be added to catch similar issues?

**Tags**: `code`, `design`  
**Best For**: Security vulnerabilities, robustness improvements.

### 8. Biology/Evolution Frame (Wild)
**Vantage**: Transplant immune system or cell signaling mechanisms into OLM.

> You are an immunologist studying how the body rejects foreign objects. How does OLM's "immune response" to bad bundles work? What happens when a bundle is "antigenically different" from expected? Can we design an "autoimmune" check that catches malformed bundles before they reach production?

**Tags**: `code`, `wild`  
**Best For**: Creative solutions, unconventional approaches.

### 9. Naive User Frame (Wild)
**Vantage**: Describe the problem without any OLM knowledge. What obvious solution is being overlooked?

> You are a 10-year-old who has never seen Kubernetes or OpenShift. Explain the problem in simple terms: "The operator won't install." What would you try first? What's the most obvious thing that grown-ups overcomplicate?

**Tags**: `general`, `wild`  
**Best For**: Overlooked simple fixes, user experience issues.

## Usage Patterns

### Pattern 1: Ambiguous Bundle Failure
```bash
/opm-troubleshooting:adhd \
  --catalog quay.io/prega/prega-operator-index:v4.22-latest \
  --package kubernetes-nmstate-operator \
  "Bundle shows PARTIAL status - missing commit SHA and repository URL in image labels"
```

**What it does**: 
1. Spawns 5 branches (Registry Auditor, Bundle Forensics, Channel Historian, etc.)
2. Each branch generates 6 hypotheses about why metadata is missing
3. Converges to top 3 with verification steps using `inspect-bundle`

### Pattern 2: Channel Migration Failure
```bash
/opm-troubleshooting:adhd \
  --catalog quay.io/prega/prega-operator-index:v4.22-latest \
  --package odf-operator \
  "Migrating from stable-4.21 to stable-4.22 causes subscription failure"
```

**What it does**:
1. Uses `resolve-channel` to verify both channels exist
2. Runs `inspect-bundle` on both channel heads for seed data
3. Diverges on breaking changes, dependency shifts, label format changes
4. Converges with migration verification steps

### Pattern 3: Batch Validation Patterns
```bash
/opm-troubleshooting:adhd \
  --list operators-to-audit.txt \
  "Multiple operators showing PARTIAL status - missing commit metadata"
```

**What it does**:
1. Runs `batch-validate` to get full picture
2. Diverges on whether pattern indicates systematic issue vs per-operator problems
3. Converges with catalog build process improvements or per-operator Dockerfile fixes

## Configuration

### Environment Variables

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

## Output Format

The command returns structured output in this order:

1. **Brief Summary** (1-2 lines): Problem and reframing used
2. **Wide Set**: Full pool grouped by cluster, each idea scored `[N7 V8 F9]`
3. **Converge**: 2-4 idea shortlist with explicit non-obvious picks marked ★
4. **Focus**: 3 deepened branches with verification steps using OPM tools
5. **Provocation**: One wildcard question opening new diagnostic direction

### Example Output Structure

```markdown
## Brief
Bundle PARTIAL status likely caused by missing `io.openshift.build.commit.id` label in base image, not operator Dockerfile.

## Wide Set (Clustered)

**Registry Label Inheritance** [N9 V7 F8]
- Base image lacks commit labels → bundle inherits empty values
- Multi-stage build drops labels in final stage
...

**Channel History Gap** [N6 V9 F9]
- Channel was updated without rebuilding bundles
- Previous maintainer skipped label addition
...

## Converge (Top 3)

★ **Registry Label Inheritance** (Novelty: 9, Viability: 7, Fit: 8)
Non-obvious because most assume operator Dockerfile is at fault.

**Verification**: Run `inspect-bundle --json` and compare base image labels vs bundle labels.

## Focus: Registry Label Inheritance

**Sketch**: Base image (e.g., `ubi8/ubi-minimal`) lacks Git metadata labels. When operator Dockerfile uses `FROM`, labels don't inherit unless explicitly copied with `COPY --from`.

**Risk**: Fixing only the operator Dockerfile won't help if base image is rebuilt without labels.

**First Step**: 
```bash
# Check base image for commit labels
skopeo inspect docker://registry.access.redhat.com/ubi8/ubi-minimal:latest | jq '.Labels'
```

**Child Ideas**:
1. Pin base image version and verify labels in CI
2. Add label validation to bundle build pipeline
3. Create a "label health check" command for catalog audits
...

## Provocation
What if OLM could detect "label drift" between channel head and previous version, alerting maintainers before subscriptions break?
```

## Performance Characteristics

| Metric | Value | Notes |
|--------|-------|-------|
| **Wall Clock** | 30-90 seconds | Depends on `OPM_AHDH_CONCURRENCY` and LLM latency |
| **LLM Calls** | ~10 per run | 5 diverge + 1 score + 1 cluster + 3 deepen |
| **Token Cost** | ~5-10x single-shot debug | Justified when cost of wrong fix is high |
| **Idea Pool** | 30 ideas (5 frames × 6 ideas) | Scales with `OPM_AHDH_TOP_K` |

## Comparison: Direct Debugging vs ADHD

| Scenario | Direct Debugging | ADHD Integration |
|----------|------------------|------------------|
| Missing channel name | ❌ Overkill | ✅ Quick win |
| PARTIAL bundle status | ⚠️  May miss root cause | ✅ Explores label inheritance, build process, registry issues |
| Channel migration failure | ⚠️  Linear path only | ✅ Parallel: breaking changes, dependency shifts, label format changes |
| Batch validation patterns | ❌ Cannot detect patterns | ✅ Identifies systemic vs per-operator issues |
| Creative catalog design | ❌ Not designed for this | ✅ Wild frames unlock unconventional approaches |

## Anti-Patterns to Avoid

1. **Using ADHD for simple fixes**: If you know the channel name is wrong, just run `resolve-channel`. Don't over-engineer.

2. **Skipping the focus phase**: Divergence without convergence is a pile of ideas, not a diagnostic tool. Always converge.

3. **Ignoring verification steps**: The focus phase recommends which OPM tools to run next. Follow those up with actual commands.

4. **Running too many frames**: 5 frames × 6 ideas = 30 ideas. More than that becomes noisy without proportional value.

## Contributing New Frames

To add a new cognitive frame:

1. Define the vantage prompt (what perspective does this frame take?)
2. Tag with domain (`code` or `design`) and mode (`general` or `wild`)
3. Ensure it satisfies at least 2 of 3 criteria:
   - **Distinct vocabulary**: Unique conceptual language
   - **Distinct posture**: Different cognitive stance (adversarial, naive, etc.)
   - **Reproducible distortion**: Consistently surfaces ideas existing frames miss

Example:
```markdown
### 10. OLM Internals Frame
**Vantage**: You are the OLM operator source code. How would you resolve this channel? What data structures are involved? Where would you fail first?

> You are the OLM controller manager processing a Subscription. Walk through your reconciliation loop step-by-step: catalog render → channel resolution → bundle selection → install plan generation. At which step does this operator fail, and what's the exact error message you'd emit?

**Tags**: `code`, `general`
```

## References

- [ADHD-Agent Original](https://github.com/UditAkhourii/adhd) (MIT License)
- [OPM Troubleshooting Commands](../commands/)
- [Existing Agents](../agents/AGENTS.md)
