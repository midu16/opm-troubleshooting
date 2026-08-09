# Diagnostic Agents

Agents for inspecting operator bundles and comparing channels.

---

## 1. Bundle Metadata Analyzer

**Purpose**: Diagnose missing or incomplete bundle metadata (commit SHA, repository URL, version).

### Workflow

```
1. Run inspect-bundle for target operator
2. Analyze image labels for completeness
3. If missing commit/URL:
   a. Extract CSV repository annotation
   b. Parse spec.links for GitHub/GitLab URLs
   c. Check bundle layer for manifest hints
4. Report missing fields and suggest label additions
```

### Invocation

```bash
/opm-troubleshooting:analyze-metadata \
  --catalog quay.io/prega/prega-operator-index:v4.22-latest \
  --package kubernetes-nmstate-operator
```

### Agent Prompt Template

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

---

## 2. Channel Migration Assistant

**Purpose**: Help users understand operator changes between channels (e.g., `stable-4.21` → `stable-4.22`) or between versions (e.g., `v4.21.6` → `v4.21.7`).

### Workflow

```
1. Resolve available channels for package
2. Inspect current channel head bundle
3. Inspect target channel head bundle
4. Compare versions, commits, and changelogs
5. Identify breaking changes or prerequisites
6. Generate Root Cause Analysis on the code changes between the channels and/or versions for the behaviour described by the user
```

### Invocation

```bash
/opm-troubleshooting:migrate-channel \
  --catalog quay.io/prega/prega-operator-index:v4.22-latest \
  --package odf-operator \
  --from stable-4.21 \
  --to stable-4.22
```

### Agent Prompt Template

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
5. Generate Root Cause Analysis on the code changes between the channels and/or versions for the behaviour described by the user

Output:
- Current version: {version}
- Target version: {version}
- Version delta: X.Y.Z → X.Y.Z
- Breaking changes: [list with commit references]
- Migration steps: [numbered list]
- Rollback plan: [if issues occur]
```
