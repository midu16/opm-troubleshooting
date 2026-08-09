# Version Resolver

**Purpose**: Find which bundle version contains a specific commit or fix.

## Workflow

```
1. Resolve all channels for package
2. For each channel, list all bundle entries (not just head)
3. Inspect each bundle for commit SHA
4. Match target commit against bundle commits
5. Report which channel and version contains the fix
```

## Invocation

```bash
/opm-troubleshooting:find-commit \
  --catalog quay.io/prega/prega-operator-index:v4.22-latest \
  --package odf-operator \
  --commit a1b2c3d4e5f6
```

## Agent Prompt Template

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
