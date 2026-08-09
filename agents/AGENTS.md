---
name: AGENTS
description: AI agent workflows for automated OLM operator troubleshooting using the opm-troubleshooting toolkit.
---

# OPM Troubleshooting Agents

AI agents for automated OLM operator troubleshooting using the `opm-troubleshooting` toolkit — a pure Go implementation with no external dependencies (no `opm`, `jq`, or `skopeo`). Combines catalog inspection, bundle analysis, must-gather fault analysis, and RAG-powered knowledge retrieval to diagnose common operator issues.

## Install

```bash
npm install -g @opm-troubleshooting/plugin
```

## Quick Commands

| Agent | Command | One-liner |
|-------|---------|-----------|
| Bundle Metadata Analyzer | `/opm-troubleshooting:analyze-metadata --catalog <img> --package <pkg>` | Diagnose missing bundle labels (commit, URL, version) |
| Channel Migration Assistant | `/opm-troubleshooting:migrate-channel --catalog <img> --package <pkg> --from <ch1> --to <ch2>` | Compare operators across channels/versions |
| Catalog Quality Auditor | `/opm-troubleshooting:audit-catalog --catalog <img> --list <file>` | Audit entire catalog for metadata quality |
| Subscription Troubleshooter | `/opm-troubleshooting:debug-subscription --subscription <yaml>` | Diagnose failing OLM subscriptions |
| Version Resolver | `/opm-troubleshooting:find-commit --catalog <img> --package <pkg> --commit <sha>` | Find which bundle contains a commit |
| Telco Diagnostician | `/opm-troubleshooting:telco-diagnose --must-gather <path> --environment <env>` | Production telco operator diagnosis (OADP, TALM, IDMS, MCH) |

## Agent Architecture

The toolkit implements a 12-step fault analysis pipeline:

| Step | Phase | Package | Purpose |
|------|-------|---------|---------|
| 0 | RCA Pattern Detection | `rca/patterns.go` | Rule-based patterns (ASYMMETRY, MISSING_GUARD, TYPE_ESCALATION, etc.) |
| 1 | Workload Parsing | `mustgather/workload.go` | Parse pods/deployments/events in target namespace |
| 2 | Health Check | `healthcheck/checker.go` | 20-dimension OLM + infrastructure checks |
| 3 | Bundle Resolution | `catalog/resolve.go`, `imageinspect/inspect.go` | Resolve installed/target bundle images, inspect labels |
| 4 | Git Delta | `gitdelta/delta.go` | Clone repos, compute diffs between commits |
| 5 | Code Analysis | `codeanalysis/analyze.go` | Search operator source for failure patterns |
| 6 | AI Analysis | `claudeapi/client.go` | Send delta + symptoms to Anthropic API |
| 7 | RAG Enrichment | `rag/engine.go`, `retrieval.go` | Vector search over docs, known issues, code paths |
| 8 | ADHD Divergence | `adhd/engine.go` | Multi-frame hypothesis generation with scoring |
| 9 | Repo Correlation | `openshift/correlator.go` | Source repo resolution + GitHub issue/PR search |
| 10 | Self-Learning | `learning/matcher.go`, `metadata/store.go` | Symptom fingerprinting, similarity search across sessions |
| 11 | RCA Report | `rca/report.go` | Compose professional Markdown report |

**Key characteristics**:
- Pure Go implementation (operator-registry + go-containerregistry) — no `opm`, `jq`, or `skopeo` required
- Four operational modes: full with Claude API, catalog-only, must-gather only, bundle inspection
- Session persistence at `~/.config/opm-troubleshooting/sessions/` across redeployments

## Agent Documentation

Detailed workflows and prompt templates are split by category:

- **[Diagnostic agents](agents/diagnostic.md)** — analyze-metadata, migrate-channel
- **[Telco Diagnostician](agents/telco.md)** — telco-diagnose + operator reference (27 packages)
- **[Catalog Audit](agents/catalog-audit.md)** — audit-catalog
- **[Subscription Troubleshooter](agents/subscription.md)** — debug-subscription
- **[Version Resolver](agents/version-resolver.md)** — find-commit

## Design Principles

1. **Agents must apply AI reasoning** beyond a single tool call — no thin wrappers around raw commands.
2. **Standard output template**: `## Summary` → `## Analysis` → `## Root Cause` → `## Recommendations` → `## References`. See [output format](guidelines/output-format.md).
3. **Graceful error handling**: network failures, auth errors, missing channels, timeouts — all must produce actionable guidance, not stack traces.

## Example Workflow

**User Request**: "Why is my cluster-logging subscription failing?"

### Agent Execution (via Claude Code)

1. User invokes `/resolve-channel --catalog redhat-operator-index:v4.22 --package cluster-logging`
2. Command renders catalog, discovers channel "stable" doesn't exist; available: stable-5.9, stable-6.0
3. User runs `/inspect-bundle --catalog ... --package cluster-logging --channel stable-5.9`
4. Agent generates fix based on bundle metadata

### Agent Response (standard format)

```markdown
## Summary
Subscription uses deprecated channel "stable" — removed in v4.22 catalog.

## Analysis
- Requested channel: `stable`
- Available channels: `stable-5.8`, `stable-5.9`, `stable-6.0`
- Default channel: `stable-5.9`

## Root Cause
cluster-logging operator migrated to version-specific channels in v5.8. Generic "stable" channel removed.

## Recommendations
1. Update Subscription manifest:
   ```yaml
   spec:
     channel: stable-5.9
   ```
2. Apply: `oc apply -f subscription.yaml`
3. Monitor: `oc get installplan -n openshift-logging`

## References
- Channel head bundle: registry.redhat.io/openshift-logging/cluster-logging-operator-bundle@sha256:...
- Version: v5.9.8
- Commit: https://github.com/openshift/cluster-logging-operator/commit/abc123
```

*See [full example](../docs/examples/subscription-failure.md) for complete workflow with all steps.*

## Integration with Claude Code

The toolkit integrates via three layers:

### 1. Plugin Slash Commands (`~/.claude-plugin/plugin.json`)

Four commands inject workflow instructions into the conversation:

| Command | Description |
|---------|-------------|
| `/inspect-bundle` | Inspect OLM catalog bundles, resolve channel heads, extract metadata |
| `/resolve-channel` | List available channels for a package in a catalog |
| `/batch-validate` | Validate multiple operators from a single catalog index |
| `/adhd` | Divergent ideation for OLM troubleshooting using parallel cognitive frames |

Each command loads its markdown file (e.g., `commands/inspect-bundle.md`) which contains structured prompts guiding Claude through the workflow.

### 2. MCP Server (`~/.mcp.json`)

Local MCP server `ocp-rag` exposes 8 tools over stdio, backed by a vector store with 5 collections:

- `ocp_docs` — OpenShift documentation
- `operator_code` — Operator source code
- `telco_configs` — Telco reference configurations
- `known_issues` — Known failure patterns
- `manifests` — Kubernetes manifests

Used for deep-troubleshooting via semantic search when standard commands don't resolve the issue.

### 3. Cursor Rules (`.cursor/rules/`)

Six rule files shape Claude's behavior:

| Rule | Scope | Purpose |
|------|-------|---------|
| `base.mdc` | All files | Project context, stack, MCP overview |
| `cli-commands.mdc` | All files | CLI usage reference for all binaries |
| `go-conventions.mdc` | `**/*.go` | Go coding conventions |
| `mcp-tools.mdc` | All files | Decision tree for using 8 OCP RAG MCP tools |
| `olm-troubleshooting.mdc` | All files | Six diagnostic workflows combining CLI + MCP |
| `rag-development.mdc` | `internal/rag/**` | RAG system architecture and dev conventions |

### Chaining Example

```bash
# 1. Discover available channels
/resolve-channel --catalog redhat-operator-index:v4.22 --package cluster-logging

# 2. Inspect channel head bundle
/inspect-bundle --catalog ... --package cluster-logging --channel stable-5.9

# 3. Deep-troubleshoot via RAG (if needed)
# Agent automatically uses MCP tools for semantic search over docs and known issues
```

## Contributing

To add a new agent: see [contributing guide](guidelines/contributing.md).
