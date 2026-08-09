# Subscription Troubleshooter

**Purpose**: Diagnose why an OLM Subscription is failing to install or upgrade.

## Workflow

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

## Invocation

```bash
/opm-troubleshooting:debug-subscription \
  --subscription subscription.yaml
```

## Agent Prompt Template

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
