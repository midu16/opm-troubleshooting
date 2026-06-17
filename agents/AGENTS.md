---
name: AGENTS
description: 
---
# OPM Troubleshooting Agents

This document defines AI agent workflows for automated OLM operator troubleshooting using the `opm-troubleshooting` toolkit.

## Overview

The `opm-troubleshooting` plugin provides specialized agents that orchestrate catalog inspection, bundle analysis, and operator debugging workflows. These agents combine the low-level inspection tools with AI reasoning to diagnose common operator issues.

## Agent Architecture

Agents in this framework follow a standard pattern:

1. **Discovery Phase**: Use catalog rendering and channel resolution to understand operator state
2. **Analysis Phase**: Inspect bundle metadata, image labels, and source repositories
3. **Diagnosis Phase**: Apply AI reasoning to identify root causes
4. **Recommendation Phase**: Suggest fixes with references to source code commits

## Available Agents

### 1. Bundle Metadata Analyzer

**Purpose**: Diagnose missing or incomplete bundle metadata (commit SHA, repository URL, version).

**Workflow**:
```
1. Run inspect-bundle for target operator
2. Analyze image labels for completeness
3. If missing commit/URL:
   a. Extract CSV repository annotation
   b. Parse spec.links for GitHub/GitLab URLs
   c. Check bundle layer for manifest hints
4. Report missing fields and suggest label additions
```

**Invocation**:
```bash
/opm-troubleshooting:analyze-metadata \
  --catalog quay.io/prega/prega-operator-index:v4.22-latest \
  --package kubernetes-nmstate-operator
```

**Agent Prompt Template**:
```
You are analyzing OLM bundle metadata for operator troubleshooting.

Context:
- Catalog: {catalog}
- Package: {package}
- Channel: {channel}

Task:
1. Inspect the bundle using inspect-bundle command
2. Identify missing metadata fields: commit, url, version, repository
3. If missing, attempt to extract from:
   - ClusterServiceVersion repository annotation
   - spec.links in CSV
   - Image label alternatives (vcs-ref, upstream-vcs-ref)
4. Provide specific recommendations for Dockerfile LABEL additions

Output format:
- Status: COMPLETE | PARTIAL | INCOMPLETE
- Missing fields: [list]
- Recommendations: [actionable steps with label examples]
```

### 2. Channel Migration Assistant

**Purpose**: Help users understand operators changes between channels (e.g., `stable-4.21` → `stable-4.22`) or between versions (e.g, `v4.21.6` → `v4.21.7`)

**Workflow**:
```
1. Resolve available channels for package
2. Inspect current channel head bundle
3. Inspect target channel head bundle
4. Compare versions, commits, and changelogs
5. Identify breaking changes or prerequisites
6. Generate Root Cause Analysis on the code changes between the channls and/or versions for the behaviour described by the user
```

**Invocation**:
```bash
/opm-troubleshooting:migrate-channel \
  --catalog quay.io/prega/prega-operator-index:v4.22-latest \
  --package odf-operator \
  --from stable-4.21 \
  --to stable-4.22
```

**Agent Prompt Template**:
```
You are assisting with OLM operator channel migration.

Context:
- Catalog: {catalog}
- Package: {package}
- Source channel: {from_channel}
- Target channel: {to_channel}

Task:
1. Use resolve-channel to list available channels
2. Use inspect-bundle for both source and target channel heads
3. Compare:
   - Version differences
   - Commit history (if GitHub URLs available)
   - Breaking changes in commit messages
4. Check for known migration issues (search operator docs)
5. Generate Root Cause Analysis on the code changes between the channls and/or versions for the behaviour described by the user

Output:
- Current version: {version}
- Target version: {version}
- Version delta: X.Y.Z → X.Y.Z
- Breaking changes: [list with commit references]
- Migration steps: [numbered list]
- Rollback plan: [if issues occur]
```

### 3. Catalog Quality Auditor

**Purpose**: Audit entire catalog for metadata quality, missing bundles, and broken channels.

**Workflow**:
```
1. Run batch-validate on operator list
2. Classify failures: FAIL vs PARTIAL
3. For FAIL: identify root cause (channel missing, bundle pull failure)
4. For PARTIAL: suggest metadata improvements
5. Generate quality report with priority ranking
```

**Invocation**:
```bash
/opm-troubleshooting:audit-catalog \
  --catalog quay.io/prega/prega-operator-index:v4.22-latest \
  --list operators.txt
```

**Agent Prompt Template**:
```
You are auditing an OLM catalog for quality and completeness.

Context:
- Catalog: {catalog}
- Operator count: {count}

Task:
1. Run batch-validate on all operators
2. Categorize results:
   - Critical failures: bundle resolution failed
   - Metadata gaps: missing commit/url (PARTIAL)
   - Success: complete metadata (OK)
3. For failures, run individual inspect-bundle to get detailed errors
4. Prioritize fixes:
   - P0: FAIL status (blocks subscription)
   - P1: PARTIAL status with missing commit (affects debugging)
   - P2: PARTIAL status with missing URL (cosmetic)

Output:
- Summary: X OK, Y PARTIAL, Z FAIL
- Critical failures: [list with error details]
- Metadata gaps: [list with recommended labels]
- Priority ranking: [P0/P1/P2 breakdown]
```

### 4. Subscription Troubleshooter

**Purpose**: Diagnose why an OLM Subscription is failing to install or upgrade.

**Workflow**:
```
1. Extract catalog, package, channel from Subscription YAML
2. Verify catalog image is accessible
3. Verify channel exists using resolve-channel
4. Inspect channel head bundle
5. Check bundle for common issues:
   - Missing required CRDs
   - Invalid CSV spec
   - Dependency conflicts
6. Suggest fixes
```

**Invocation**:
```bash
/opm-troubleshooting:debug-subscription \
  --subscription subscription.yaml
```

**Agent Prompt Template**:
```
You are troubleshooting an OLM Subscription installation failure.

Context:
- Subscription: {name}
- Namespace: {namespace}
- Catalog: {catalogSource}
- Package: {package}
- Channel: {channel}

Task:
1. Verify catalog image exists and is pullable
2. Run resolve-channel to confirm channel exists
3. If channel missing:
   - List available channels
   - Suggest correct channel or defaultChannel
4. Run inspect-bundle on channel head
5. If bundle inspection fails:
   - Check bundle image pullability
   - Verify bundle format
6. Check for known issues:
   - CRD version compatibility
   - RBAC requirements
   - Dependency resolution

Output:
- Status: CATALOG_ISSUE | CHANNEL_ISSUE | BUNDLE_ISSUE | UNKNOWN
- Root cause: [detailed explanation]
- Fix: [specific YAML changes or commands]
- References: [docs, GitHub issues, commit URLs]
```

### 5. Version Resolver

**Purpose**: Find which bundle version contains a specific commit or fix.

**Workflow**:
```
1. Resolve all channels for package
2. For each channel, list all bundle entries (not just head)
3. Inspect each bundle for commit SHA
4. Match target commit against bundle commits
5. Report which channel and version contains the fix
```

**Invocation**:
```bash
/opm-troubleshooting:find-commit \
  --catalog quay.io/prega/prega-operator-index:v4.22-latest \
  --package odf-operator \
  --commit a1b2c3d4e5f6
```

**Agent Prompt Template**:
```
You are searching for which operator bundle contains a specific commit.

Context:
- Catalog: {catalog}
- Package: {package}
- Target commit: {commit_sha}

Task:
1. Use resolve-channel to get all channels
2. For each channel:
   - Inspect all bundle entries (not just head)
   - Extract commit SHA from each bundle
   - Check if target commit is present
3. If commit not found in any bundle:
   - Check if commit exists in upstream repository
   - Determine if commit is in unreleased version
4. Report channel and version containing commit

Output:
- Found: YES | NO
- Channel: {channel_name}
- Bundle version: {version}
- Bundle image: {image_ref}
- Commit URL: {url}
- If not found: [explanation and estimated release timeline]
```

## Agent Implementation Guidelines

### Using AI Reasoning

Agents should apply AI analysis for:
- **Pattern Recognition**: Identify common failure modes (e.g., "channel X.Y always missing commit labels")
- **Context Synthesis**: Combine catalog metadata with external docs (operator repos, Red Hat docs)
- **Error Interpretation**: Translate technical errors into actionable user guidance
- **Recommendation Ranking**: Prioritize fixes by impact and effort

### Avoiding Pure Automation

Do NOT create agents for simple command wrappers. These should be direct commands:
- ❌ **Bad**: `/opm-troubleshooting:inspect` that just calls `inspect-bundle` with no analysis
- ✅ **Good**: `/opm-troubleshooting:analyze-metadata` that interprets missing fields and suggests fixes

### Error Handling

Agents must gracefully handle:
- **Network failures**: Suggest retry with different registry mirrors
- **Authentication errors**: Guide user to set `DOCKER_CONFIG`
- **Missing channels**: List available channels and suggest alternatives
- **Timeout issues**: Recommend increasing `--timeout` or splitting work

### Output Format

Agent outputs should follow this structure:
```
## Summary
[1-2 sentence overview]

## Analysis
[Detailed findings with data]

## Root Cause
[Specific diagnosis with references]

## Recommendations
1. [Action item with command/YAML]
2. [Action item with command/YAML]

## References
- [Commit URL]
- [Documentation link]
- [GitHub issue]
```

## Integration with Claude Code

Agents integrate with Claude Code workflows:

```bash
# Direct invocation
/opm-troubleshooting:audit-catalog --catalog quay.io/... --list operators.txt

# Chaining with other skills
/opm-troubleshooting:debug-subscription --subscription sub.yaml
# Agent identifies missing channel, then:
/opm-troubleshooting:resolve-channel --catalog ... --package ...

# Background execution for large audits
/opm-troubleshooting:audit-catalog --catalog ... --list 500-operators.txt --background
```

## Example Agent Workflow

**User Request**: "Why is my cluster-logging subscription failing?"

**Agent Execution**:
1. Parse Subscription YAML (catalog: redhat-operator-index, channel: stable)
2. Run `/opm-troubleshooting:resolve-channel --catalog ... --package cluster-logging`
3. Discover channel "stable" doesn't exist; available: stable-5.9, stable-6.0
4. Run `/opm-troubleshooting:inspect-bundle` on stable-5.9 (latest)
5. Generate fix:
   ```yaml
   # Update Subscription channel
   spec:
     channel: stable-5.9  # Changed from "stable"
   ```

**Agent Response**:
```
## Summary
Subscription is using deprecated channel "stable" which no longer exists in v4.22 catalog.

## Analysis
- Current catalog: redhat-operator-index:v4.22
- Requested channel: stable
- Available channels: stable-5.8, stable-5.9, stable-6.0
- Default channel: stable-5.9

## Root Cause
The cluster-logging operator migrated to version-specific channels in v5.8.
The generic "stable" channel was removed. Subscriptions must now specify
a versioned channel (e.g., stable-5.9).

## Recommendations
1. Update Subscription manifest:
   ```yaml
   spec:
     channel: stable-5.9
   ```
2. Apply the change: `oc apply -f subscription.yaml`
3. Monitor InstallPlan: `oc get installplan -n openshift-logging`

## References
- Channel head bundle: registry.redhat.io/openshift-logging/cluster-logging-operator-bundle@sha256:...
- Version: v5.9.8
- Commit: https://github.com/openshift/cluster-logging-operator/commit/abc123
- Docs: https://docs.openshift.com/container-platform/4.22/logging/cluster-logging-deploying.html
```

## Future Enhancements

Planned agent capabilities:
- **Dependency Graph Analyzer**: Visualize operator dependency chains
- **Performance Profiler**: Analyze bundle inspection latency across catalogs
- **Security Scanner**: Check bundle images for CVEs and outdated base images
- **Upgrade Path Optimizer**: Suggest optimal channel migration sequences
- **Catalog Diff Tool**: Compare two catalog versions and highlight changes

## Contributing New Agents

When adding new agents:
1. Define clear use case requiring AI reasoning (not simple scripting)
2. Document workflow steps in this file
3. Create command markdown in `commands/` directory
4. Add agent prompt template
5. Test against real catalog scenarios
6. Update plugin version in `.claude-plugin/plugin.json`
