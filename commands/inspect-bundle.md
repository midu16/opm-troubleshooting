---
description: Inspect OLM catalog bundle metadata and resolve channel head bundles
argument-hint: "--catalog <image> --package <name> [--channel <name>] [--version <ver>]"
---

## Name
opm-troubleshooting:inspect-bundle

## Synopsis
```
/opm-troubleshooting:inspect-bundle --catalog <catalog-image> --package <package-name> [--channel <channel-name>] [--version <version>]
```

## Description
The `opm-troubleshooting:inspect-bundle` command renders an OLM catalog index image, resolves the channel-head bundle (or a specific version) for a given package and channel, and inspects the bundle image metadata including commit SHA, repository URL, and version information.

This command provides a **pure Go implementation** that replaces the traditional shell toolchain (`opm render` + `jq` + `skopeo inspect`) with native Go libraries:
- Uses `operator-registry` action.Render for catalog rendering
- Uses `declcfg.DeclarativeConfig` for in-memory FBC processing
- Uses `go-containerregistry` for bundle image inspection
- Parses bundle CSV manifests to extract repository and commit URLs

The tool is designed for operator troubleshooting workflows where you need to quickly identify:
- Which bundle version is deployed on a channel
- The source code commit that produced a bundle
- Repository URLs for filing issues or reviewing code
- Bundle metadata for debugging OLM issues

## Implementation

The command executes the following workflow:

1. **Catalog Rendering**: Pulls and renders the catalog index image using `operator-registry/alpha/action.Render`
2. **Channel Resolution**: Traverses the FBC declarative config to find the package and channel
3. **Bundle Selection**: 
   - If `--version` is omitted: selects the last entry in `olm.channel.entries` (channel head)
   - If `--version` is provided: matches bundle version on the channel
4. **Image Inspection**: Pulls bundle image config and extracts labels using `go-containerregistry`
5. **Commit Resolution**: Reads commit SHA from image labels in this order:
   - `io.openshift.build.commit.id` (OpenShift builds)
   - `org.opencontainers.image.revision` (OCI standard)
   - `vcs-ref` or `upstream-vcs-ref` (legacy labels)
6. **URL Resolution**: Builds commit URL from:
   - `io.openshift.build.commit.url` (direct URL label)
   - Repository labels (`org.opencontainers.image.source`, `io.openshift.build.source-location`)
   - CSV repository annotation or spec.links
   - Composes GitHub/GitLab commit URLs

The tool respects registry authentication via `DOCKER_CONFIG` environment variable.

## Return Value
- **Claude agent text**: JSON or text output containing:
  - `package`: Package name
  - `bundle`: Bundle image reference (tag or digest)
  - `version`: Bundle version from image labels
  - `commit`: Git commit SHA
  - `url`: GitHub/GitLab commit URL

Exit codes:
- `0`: Success
- `1`: Usage error or bundle resolution failure
- `2`: Catalog render or image inspect failure

## Examples

### Example 1: Inspect channel head bundle (default channel)
```bash
/opm-troubleshooting:inspect-bundle \
  --catalog quay.io/prega/prega-operator-index:v4.22-20260607T194312 \
  --package kubernetes-nmstate-operator
```

Output:
```
package: kubernetes-nmstate-operator
bundle:  registry.redhat.io/openshift4/kubernetes-nmstate-operator-bundle@sha256:abc123...
version: v4.22.0-202606071943
commit:  a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0
url:     https://github.com/nmstate/kubernetes-nmstate/commit/a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0
```

### Example 2: Inspect specific channel
```bash
/opm-troubleshooting:inspect-bundle \
  --catalog quay.io/prega/prega-operator-index:v4.22-20260607T194312 \
  --package odf-operator \
  --channel stable-4.21
```

### Example 3: Inspect specific version on a channel
```bash
/opm-troubleshooting:inspect-bundle \
  --catalog quay.io/prega/prega-operator-index:v4.22-20260607T194312 \
  --package odf-operator \
  --channel stable-4.21 \
  --version v4.21.7
```

### Example 4: JSON output for scripting
```bash
/opm-troubleshooting:inspect-bundle \
  --catalog quay.io/prega/prega-operator-index:v4.22-20260607T194312 \
  --package advanced-cluster-management \
  --channel release-2.11 \
  --json
```

Output:
```json
{
  "package": "advanced-cluster-management",
  "bundle": "registry.redhat.io/rhacm2/acm-operator-bundle@sha256:def456...",
  "version": "v2.11.2",
  "commit": "5d1e5bd5b55d8ba2fd2a2c1199ff819e9537ea25",
  "url": "https://github.com/stolostron/acm-operator/commit/5d1e5bd5b55d8ba2fd2a2c1199ff819e9537ea25"
}
```

## Arguments

- `--catalog` (required): OLM catalog index image reference (e.g., `quay.io/prega/prega-operator-index:v4.22-latest`)
- `--package` (required): Operator package name as defined in the catalog
- `--channel` (optional): Channel name to inspect. If omitted, uses the package's `defaultChannel`
- `--version` (optional): Specific bundle version to resolve on the channel (e.g., `v4.21.7`). If omitted, returns the channel head (last entry)
- `--json` (optional): Output results in JSON format instead of text
- `--timeout` (optional): Overall operation timeout (default: 10m)

## Environment Variables

- `DOCKER_CONFIG`: Path to directory containing `config.json` for registry authentication
- `REGISTRY_AUTH_FILE`: Alternative registry credentials file (Docker/Podman compatible)

## Notes

- The command requires network access to pull catalog and bundle images
- Channel head is determined by FBC entry order, **not semver sorting**
- For private catalogs, ensure `DOCKER_CONFIG` points to valid credentials
- Commit URLs are best-effort: some bundles may lack sufficient labels for URL construction
- The tool can extract repository URLs from ClusterServiceVersion manifests when image labels are insufficient

## See Also

- `opm-troubleshooting:batch-validate` - Validate multiple operators from a catalog
- `opm-troubleshooting:resolve-channel` - List available channels for a package
