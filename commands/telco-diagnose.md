---
description: Production-grade telco operator diagnosis for OADP, TALM, IDMS, and MCH
argument-hint: "--must-gather <path> [--environment lab|disconnected|kvm|production] [--rca-file <path>]"
---

## Name
opm-troubleshooting:telco-diagnose

## Synopsis
```
/opm-troubleshooting:telco-diagnose --must-gather <path> [options]
```

## Description
The `opm-troubleshooting:telco-diagnose` command performs production-grade diagnosis of the telco operator suite on OpenShift clusters. It replaces hours of manual investigation with systematic, repeatable analysis in minutes.

**Telco suite operators (27 OLM + IDMS):**

| Category | Operators |
|----------|-----------|
| Cluster | `advanced-cluster-management`, `multicluster-engine` |
| Lifecycle | `topology-aware-lifecycle-manager`, `lifecycle-agent`, `openshift-gitops-operator` |
| Logging | `cluster-logging` |
| Networking | `kubernetes-nmstate-operator`, `metallb-operator`, `sriov-network-operator`, `ptp-operator`, `numaresources-operator` |
| Storage | `local-storage-operator`, `lvms-operator` |
| ODF | `odf-operator`, `odf-dependencies`, `rook-ceph-operator`, `cephcsi-operator`, `ocs-operator`, `ocs-client-operator`, `mcg-operator`, `odf-csi-addons-operator`, `odf-external-snapshotter-operator`, `odf-prometheus-operator`, `recipe` |
| Backup | `redhat-oadp-operator` |
| Config | ImageDigestMirrorSet, `o-cloud-manager`, `openshift-cert-manager-operator` |

**Capabilities:**
- **20-dimension health checks** — OLM chain, workloads, events, telco-specific dimensions on every run
- **Noise filtering** — Distinguishes real faults from cosmetic KVM/lab/disconnected alerts
- **Code-level analysis** — Correlates failure symptoms with operator source code (local repo or git clone)
- **Professional RCA** — Shareable Markdown report with executive summary, evidence, and actions
- **Session continuity** — Tracks redeployment history; no need to repeat context

## Implementation

Executes `telco-diagnose` binary (or `catalog-bundle-inspect --telco-suite` equivalent):

1. **Parse must-gather** — Extract OLM state, workloads, events, IDMS resources
2. **Health check** — Run 20 systematic dimensions per operator
3. **Noise filter** — Classify findings as real/cosmetic/ambiguous based on `--environment`
4. **Bundle analysis** — Resolve installed vs target bundles from catalog (optional)
5. **Code correlation** — Search operator source for failure patterns
6. **RCA generation** — Produce professional Markdown report
7. **Session update** — Persist redeployment context for next run

## Agent Workflow

When invoked as an agent command, follow this workflow:

```
1. Verify prerequisites:
   - must-gather path exists
   - DOCKER_CONFIG set (if using --catalog)
   - ANTHROPIC_API_KEY set (optional, for AI code delta)

2. Run telco-diagnose:
   ./bin/telco-diagnose \
     --must-gather $MUST_GATHER \
     --environment $ENVIRONMENT \
     --cluster-name $CLUSTER \
     --catalog $CATALOG \
     --rca-file /tmp/telco-rca.md \
     --json

3. Interpret results:
   - Prioritize [REAL] findings over [COSMETIC]
   - Cross-reference code-level evidence with root cause
   - Check redeployment history for regressions

4. Present to user:
   - Executive summary (1-2 sentences)
   - Real issues requiring action (numbered, prioritized)
   - Cosmetic alerts to ignore (with noise reason)
   - Link to full RCA markdown
```

## Return Value

- **JSON** (with `--json`): Full analysis result including health dimensions, noise classification, code matches, RCA document
- **Human-readable**: Per-operator fault reports with health summary, noise filter, code evidence
- **RCA file** (with `--rca-file`): Professional Markdown suitable for sharing with stakeholders

Exit codes:
- `0`: Analysis completed (may include real issues found)
- `1`: Usage error
- `2`: Must-gather parse or analysis failure

## Examples

### Full telco suite — lab environment
```bash
/opm-troubleshooting:telco-diagnose \
  --must-gather /path/to/must-gather.local.123456 \
  --environment lab \
  --cluster-name edge-lab-01 \
  --catalog registry.redhat.io/redhat/redhat-operator-index:v4.22 \
  --rca-file /tmp/telco-rca.md
```

### Single operator — disconnected cluster
```bash
/opm-troubleshooting:telco-diagnose \
  --must-gather /path/to/must-gather \
  --package redhat-oadp-operator \
  --environment disconnected \
  --source-repo ~/src/oadp-operator \
  --rca-file /tmp/oadp-rca.md
```

### Redeployment iteration
```bash
/opm-troubleshooting:telco-diagnose \
  --must-gather /path/to/must-gather-redeploy-3 \
  --cluster-name production-hub-01 \
  --state-dir ~/.config/opm-troubleshooting/sessions \
  --rca-file /tmp/redeploy-3-rca.md
```

## Arguments

- `--must-gather` (required): Must-gather directory or glob pattern
- `--catalog` (optional): Catalog index for bundle metadata and code delta
- `--package` (optional): Single operator package (default: full telco suite)
- `--version` (optional): Target bundle version for code comparison
- `--environment` (optional): `production`, `disconnected`, `lab`, `kvm` (default: production)
- `--source-repo` (optional): Local operator source repo path
- `--rca-file` (optional): Write RCA markdown to file
- `--cluster-name` (optional): Cluster identifier for session persistence
- `--state-dir` (optional): Session store directory
- `--no-rca` (optional): Skip RCA generation
- `--no-health-check` (optional): Skip 20-dimension checks
- `--json` (optional): JSON output
- `--timeout` (optional): Timeout (default 10m)

## Environment Variables

- `DOCKER_CONFIG`: Registry credentials for catalog/bundle pulls
- `ANTHROPIC_API_KEY`: Optional AI code-change correlation
- `REGISTRY_AUTH_FILE`: Alternative registry auth

## See Also

- `opm-troubleshooting:inspect-bundle` — Bundle metadata inspection
- Agent: **Telco Diagnostician** (see agents/AGENTS.md)
