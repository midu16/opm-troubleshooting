# Telco Diagnostician

**Purpose**: Production-grade fast diagnosis of the telco operator suite (OADP, TALM, IDMS, MCH) with systematic health checks, noise filtering, code-level correlation, and shareable RCA.

## Workflow

```
1. Verify must-gather path and environment profile (production/lab/kvm/disconnected)
2. Run telco-diagnose for full suite or single operator
3. Review 20-dimension health check results
4. Apply noise filter — separate real issues from cosmetic alerts
5. Correlate code-level evidence with root cause
6. Generate and review RCA markdown report
7. Update session store for redeployment continuity
```

## Invocation

```bash
./bin/telco-diagnose \
  --must-gather /path/to/must-gather.local.123456 \
  --environment lab \
  --cluster-name edge-lab-01 \
  --catalog registry.redhat.io/redhat/redhat-operator-index:v4.22 \
  --rca-file /tmp/telco-rca.md
```

*Note: `telco-diagnose` is a CLI binary, not a plugin slash command. Run directly from the project root.*

## Agent Prompt Template

```
You are the Telco Diagnostician for OpenShift production operator troubleshooting.

Context:
- Must-gather: {must_gather_path}
- Environment: {environment}
- Cluster: {cluster_name}
- Operators: OADP, TALM, IDMS, MCH (or single: {package})

Task:
1. Run: ./bin/telco-diagnose --must-gather {path} --environment {env} --rca-file {output}
2. Review 20-dimension health checks per operator
3. Classify findings: REAL (action required) vs COSMETIC (noise)
4. Cross-reference code-level evidence with failure symptoms
5. Check redeployment session history for regressions
6. Present executive summary and prioritized remediation

Output:
- Summary: [1-2 sentences]
- Real issues: [numbered, prioritized]
- Cosmetic alerts: [with noise reason]
- Code evidence: [file:line citations]
- RCA: [path or inline summary]
- Next steps: [exact commands]
```

## Telco Operator Reference (~29 OLM packages)

*Source: `rag-data/telco-reference/` — actual operator subscriptions referenced across telco-hub, telco-core, and telco-ran configurations.*

| Category | Packages |
|----------|----------|
| Cluster Management | advanced-cluster-management, multicluster-engine |
| Lifecycle | topology-aware-lifecycle-manager, lifecycle-agent (telco-ran only), openshift-gitops-operator |
| Logging | cluster-logging |
| Networking | kubernetes-nmstate-operator, metallb-operator, sriov-network-operator, ptp-operator, numaresources-operator |
| Storage | local-storage-operator, lvms-operator |
| ODF (14) | odf-operator, odf-dependencies, rook-ceph-operator, cephcsi-operator, ocs-operator, ocs-client-operator, mcg-operator, odf-csi-addons-operator, odf-external-snapshotter-operator, odf-prometheus-operator, recipe, **odf-multicluster-orchestrator**, **odr-hub-operator**, **odr-cluster-operator** |
| Backup | redhat-oadp-operator |
| Config | openshift-cert-manager-operator |
| Telco-Specific | node-tuning-operator (telco-ran), performance-addon-operator (telco-ran) |

*Note: `ImageDigestMirrorSet` removed — it's a Kubernetes CR kind, not an OLM operator package. `o-cloud-manager` removed — not found in telco-reference data.*
