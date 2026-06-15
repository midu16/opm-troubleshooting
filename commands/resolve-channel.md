---
description: Resolve OLM package channels and discover available upgrade paths
argument-hint: "--catalog <image> --package <name>"
---

## Name
opm-troubleshooting:resolve-channel

## Synopsis
```
/opm-troubleshooting:resolve-channel --catalog <catalog-image> --package <package-name>
```

## Description
The `opm-troubleshooting:resolve-channel` command renders an OLM catalog index image and lists all available channels for a given package, including the default channel and channel entries (bundle versions). This is essential for understanding operator upgrade paths and troubleshooting channel-related issues.

This command helps answer questions like:
- What channels are available for this operator?
- What is the default channel for auto-subscription?
- What bundles are available on a specific channel?
- How does the channel upgrade graph look?

The command uses pure Go implementation via `operator-registry` libraries to render FBC (File-Based Catalog) metadata and traverse the declarative config structure.

## Implementation

The command executes the following workflow:

1. **Catalog Rendering**: Pulls and renders the catalog index image using `operator-registry/alpha/action.Render`
2. **Package Discovery**: Searches `cfg.Packages` for the specified package name
3. **Channel Enumeration**: Iterates through `cfg.Channels` and filters by package name
4. **Default Channel Resolution**: Extracts `defaultChannel` from the package metadata
5. **Channel Entry Listing**: For each channel, lists all bundle entries in FBC order

The output shows:
- Package name
- Default channel (used when no channel is specified in Subscription)
- All available channels with their entry count
- Optionally: detailed bundle list per channel

## Return Value
- **Claude agent text**: Text or JSON output containing:
  - `package`: Package name
  - `defaultChannel`: The default channel name
  - `channels`: Array of channel objects with names and bundle entries

## Examples

### Example 1: List channels for a package
```bash
/opm-troubleshooting:resolve-channel \
  --catalog quay.io/prega/prega-operator-index:v4.22-20260607T194312 \
  --package odf-operator
```

Output:
```
package: odf-operator
defaultChannel: stable-4.22

Available channels:
- stable-4.21 (15 bundles)
- stable-4.22 (8 bundles)
- stable-4.23 (3 bundles)
```

### Example 2: Show detailed bundle entries
```bash
/opm-troubleshooting:resolve-channel \
  --catalog quay.io/prega/prega-operator-index:v4.22-20260607T194312 \
  --package kubernetes-nmstate-operator \
  --detailed
```

Output:
```
package: kubernetes-nmstate-operator
defaultChannel: stable

Channel: stable
  1. kubernetes-nmstate-operator.v4.22.0-202601011200
  2. kubernetes-nmstate-operator.v4.22.0-202602011200
  3. kubernetes-nmstate-operator.v4.22.0-202603011200
  4. kubernetes-nmstate-operator.v4.22.0-202604011200
  → kubernetes-nmstate-operator.v4.22.0-202605011200 (HEAD)

Channel: stable-4.21
  1. kubernetes-nmstate-operator.v4.21.0-202512011200
  → kubernetes-nmstate-operator.v4.21.0-202601011200 (HEAD)
```

### Example 3: JSON output for automation
```bash
/opm-troubleshooting:resolve-channel \
  --catalog quay.io/prega/prega-operator-index:v4.22-20260607T194312 \
  --package advanced-cluster-management \
  --json
```

Output:
```json
{
  "package": "advanced-cluster-management",
  "defaultChannel": "release-2.11",
  "channels": [
    {
      "name": "release-2.10",
      "entries": ["acm.v2.10.0", "acm.v2.10.1", "acm.v2.10.2"],
      "head": "acm.v2.10.2"
    },
    {
      "name": "release-2.11",
      "entries": ["acm.v2.11.0", "acm.v2.11.1"],
      "head": "acm.v2.11.1"
    }
  ]
}
```

### Example 4: Troubleshoot missing channel error
```bash
/opm-troubleshooting:resolve-channel \
  --catalog quay.io/prega/prega-operator-index:v4.22-20260607T194312 \
  --package cluster-logging
```

This helps debug errors like:
```
Error: channel "stable" not found for package "cluster-logging" (defaultChannel: stable-5.9); available channels: stable-5.8, stable-5.9, stable-6.0
```

## Arguments

- `--catalog` (required): OLM catalog index image reference
- `--package` (required): Operator package name as defined in the catalog
- `--detailed` (optional): Show full bundle entry lists for each channel
- `--json` (optional): Output results in JSON format
- `--timeout` (optional): Operation timeout (default: 10m)

## Environment Variables

- `DOCKER_CONFIG`: Path to directory containing `config.json` for registry authentication
- `REGISTRY_AUTH_FILE`: Alternative registry credentials file

## Notes

- Channel head is always the **last entry** in `olm.channel.entries` (FBC order, not semver)
- The default channel is used when creating a Subscription without specifying `spec.channel`
- Some packages may not define a default channel (older catalog formats)
- Channel names often follow patterns like `stable`, `stable-4.X`, `candidate`, `fast`

## Use Cases

1. **Subscription Troubleshooting**: Verify the channel exists before creating a Subscription
2. **Upgrade Planning**: Understand available upgrade paths between versions
3. **Channel Migration**: Identify bundles when moving between channels (e.g., `stable-4.21` → `stable-4.22`)
4. **Catalog Validation**: Ensure expected channels are present in a catalog build
5. **Version Discovery**: Find which channel contains a specific operator version

## See Also

- `opm-troubleshooting:inspect-bundle` - Inspect a specific bundle on a channel
- `opm-troubleshooting:batch-validate` - Validate multiple operators
