# GitHub Actions Workflows

This document describes the GitHub Actions workflows configured for this project.

## Overview

The project uses two main workflows:
1. **CI Workflow** (`.github/workflows/ci.yml`) - Runs on every push and pull request
2. **Release Workflow** (`.github/workflows/release.yml`) - Runs on version tags

## CI Workflow

### Triggers
- Push to `main` or `master` branches
- Pull requests to any branch

### Jobs

#### 1. Test Job
Runs linting, building, and testing:

- **Platform**: Ubuntu Latest
- **Node.js**: Force Node.js 24 (via `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24=true`)
- **Steps**:
  1. Checkout code
  2. Setup Go (version from go.mod)
  3. Install libgpgme-dev dependencies
  4. Build binary
  5. Run unit tests
  6. Run functional tests
  7. Run linter (golangci-lint)

#### 2. Build Binaries Job
Builds cross-platform binaries after tests pass:

- **Depends on**: `test` job
- **Platforms**:
  - Linux AMD64 (x86_64)
  - Linux ARM64 (aarch64)
- **Build Configuration**:
  - `CGO_ENABLED=0` - Pure Go build, no C dependencies
  - `-tags containers_image_openpgp` - Use OpenPGP instead of gpgme
  - `-ldflags "-s -w"` - Strip debug info and reduce binary size
- **Artifacts**:
  - Binary: `catalog-bundle-inspect-{platform}`
  - Checksum: `catalog-bundle-inspect-{platform}.sha256`
  - Retention: 30 days

### Downloading CI Artifacts

After a successful CI run:

```bash
# Using GitHub CLI
gh run list --workflow=ci.yml --limit 1
gh run download <run-id> -n catalog-bundle-inspect-linux-amd64

# Or via web UI
# Navigate to: Actions → CI → Latest run → Artifacts
```

## Release Workflow

### Triggers
- Push of version tags (e.g., `v1.0.0`, `v1.2.3`)
- Manual trigger via `workflow_dispatch`

### Platforms

Builds for three architectures:
- Linux AMD64 (`linux-amd64`)
- Linux ARM64 (`linux-arm64`)
- Linux 386 (`linux-386`)

### Build Process

For each platform:
1. **Build Binary**
   - Extract version from tag (e.g., `refs/tags/v1.0.0` → `v1.0.0`)
   - Inject version into binary: `-ldflags "-X main.version=${VERSION}"`
   - Output: `catalog-bundle-inspect-{platform}`

2. **Create Tarball**
   - Compress binary: `catalog-bundle-inspect-{platform}.tar.gz`
   - Generate checksum: `catalog-bundle-inspect-{platform}.tar.gz.sha256`

3. **Upload to Release**
   - Attach tarball and checksum to GitHub release
   - Release is public (not draft, not prerelease)

### Creating a Release

```bash
# Tag a release
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# GitHub Actions will automatically:
# 1. Build binaries for all platforms
# 2. Create GitHub release
# 3. Upload tarballs and checksums
```

### Downloading Release Binaries

```bash
# Download latest release for Linux AMD64
curl -L -o catalog-bundle-inspect.tar.gz \
  https://github.com/midu16/opm-troubleshooting/releases/latest/download/catalog-bundle-inspect-linux-amd64.tar.gz

# Download checksum
curl -L -o catalog-bundle-inspect.tar.gz.sha256 \
  https://github.com/midu16/opm-troubleshooting/releases/latest/download/catalog-bundle-inspect-linux-amd64.tar.gz.sha256

# Verify checksum
sha256sum -c catalog-bundle-inspect.tar.gz.sha256

# Extract
tar -xzf catalog-bundle-inspect.tar.gz

# Install
sudo mv catalog-bundle-inspect-linux-amd64 /usr/local/bin/catalog-bundle-inspect
chmod +x /usr/local/bin/catalog-bundle-inspect
```

## Node.js 24 Migration

### Background
GitHub Actions deprecated Node.js 20 with the following timeline:
- **June 16, 2026**: Node.js 24 becomes default
- **September 16, 2026**: Node.js 20 removed from runners

### Solution
Both workflows set `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24=true` to:
1. ✅ Prevent deprecation warnings
2. ✅ Ensure compatibility with Node.js 24
3. ✅ Future-proof workflows

### Actions Using Node.js
- `actions/checkout@v6` - Compatible with Node.js 24
- `actions/setup-go@v5` - Compatible with Node.js 24
- `actions/upload-artifact@v4` - Compatible with Node.js 24
- `softprops/action-gh-release@v2` - Compatible with Node.js 24

## Build Configuration

### Cross-Compilation

The workflows use Go's cross-compilation capabilities:

```yaml
env:
  GOOS: linux          # Target OS
  GOARCH: amd64        # Target architecture
  CGO_ENABLED: 0       # Disable C dependencies
```

### Why CGO_ENABLED=0?

Benefits:
- ✅ **Portability**: Binary runs on any Linux system (no libc dependencies)
- ✅ **Simplicity**: No need to install C libraries in CI
- ✅ **Size**: Smaller binaries (static linking)
- ✅ **Speed**: Faster builds (no C compilation)

Trade-off:
- ❌ Cannot use C libraries (but we use `containers_image_openpgp` instead of gpgme)

### Binary Size Optimization

```bash
-ldflags "-s -w"
```

- `-s`: Strip symbol table
- `-w`: Strip DWARF debug info

**Result**: ~40% smaller binaries (60MB → 40MB)

## Testing Workflows Locally

### Using `act`

Install [act](https://github.com/nektos/act) to run GitHub Actions locally:

```bash
# Install act
curl https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash

# Run CI workflow
act push -W .github/workflows/ci.yml

# Run specific job
act -j test

# Dry run (list jobs)
act -l
```

### Manual Testing

```bash
# Simulate CI build
make build
make test
make test-functional
make lint

# Simulate release build
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go build -tags containers_image_openpgp \
  -ldflags "-s -w -X main.version=v1.0.0-dev" \
  -o bin/catalog-bundle-inspect-linux-amd64 \
  ./cmd/catalog-bundle-inspect

# Generate checksum
sha256sum bin/catalog-bundle-inspect-linux-amd64 > bin/catalog-bundle-inspect-linux-amd64.sha256
```

## Troubleshooting

### Issue: Node.js 20 Deprecation Warning

**Error**:
```
Node.js 20 actions are deprecated. The following actions are running on Node.js 20...
```

**Fix**: Ensure `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24=true` is set in workflow env.

### Issue: CGO Build Failure

**Error**:
```
fatal error: gpgme.h: No such file or directory
```

**Fix**: Ensure `CGO_ENABLED=0` and `-tags containers_image_openpgp` are set.

### Issue: Cross-Compilation Fails

**Error**:
```
unsupported GOOS/GOARCH pair linux/arm64
```

**Fix**: Ensure Go 1.17+ is installed (check `go.mod`).

### Issue: Artifact Upload Fails

**Error**:
```
Unable to find any artifacts for the associated workflow
```

**Fix**:
1. Verify `dist/` directory is created
2. Check file paths in workflow YAML
3. Ensure job completed successfully

## Security Considerations

### Permissions

CI workflow uses minimal permissions:
```yaml
permissions:
  contents: read  # Read-only access
```

Release workflow requires write access:
```yaml
permissions:
  contents: write  # Required to create releases
```

### Secrets

No secrets are currently required. Future considerations:
- **Docker Hub**: For container image publishing
- **Signing Keys**: For binary signing (cosign, GPG)
- **Slack/Email**: For build notifications

### Binary Verification

Users should verify checksums:
```bash
sha256sum -c catalog-bundle-inspect-linux-amd64.tar.gz.sha256
```

Future enhancement: Sign binaries with GPG or cosign for cryptographic verification.

## Future Enhancements

### Potential Improvements

1. **Container Images**
   - Build and push Docker images to ghcr.io
   - Multi-arch manifests (amd64, arm64)

2. **Binary Signing**
   - Sign releases with GPG
   - Use cosign for keyless signing

3. **Homebrew Formula**
   - Auto-update homebrew-core formula on release

4. **Benchmarking**
   - Run performance benchmarks in CI
   - Track performance over time

5. **Code Coverage**
   - Upload coverage to Codecov/Coveralls
   - Add coverage badge to README

6. **Notifications**
   - Slack notifications on build failures
   - Email on release success

## References

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Node.js 20 Deprecation Notice](https://github.blog/changelog/2025-09-19-deprecation-of-node-20-on-github-actions-runners/)
- [Go Cross-Compilation](https://go.dev/doc/install/source#environment)
- [action-gh-release](https://github.com/softprops/action-gh-release)
- [actions/upload-artifact](https://github.com/actions/upload-artifact)
