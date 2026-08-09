# Example: Subscription Failure Workflow

**User Request**: "Why is my cluster-logging subscription failing?"

## Agent Execution

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

## Agent Response

```markdown
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
