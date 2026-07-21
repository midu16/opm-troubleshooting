# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Pre-commit hooks with gitleaks for secret scanning
- Dependabot configuration for Go modules and GitHub Actions
- Security scanning workflow (govulncheck, gitleaks, Trivy)
- Dockerfile for containerized builds
- Issue templates and PR template
- CONTRIBUTING.md, CODE_OF_CONDUCT.md, SECURITY.md
- EditorConfig for consistent formatting
- CODEOWNERS for automatic review assignment
- GoReleaser configuration for standardized releases
- Code coverage reporting with Codecov integration

### Changed
- Enhanced golangci-lint configuration with additional linters
- Improved CI workflow with coverage reporting

## [1.1.0] - 2025-06-15

### Added
- ADHD divergent ideation framework with OLM-specific cognitive frames
- Live cluster diagnostics via `oc` and `omc`
- Metadata store for persistent session tracking
- Self-learning pattern recognition with fingerprinting
- OpenShift repository correlation engine
- Telco operator diagnostics (`telco-diagnose`)
- Must-gather fault analysis and RCA patterns
- Batch validation tool (`batch-validate`)

### Changed
- Migrated CI to Node.js 24 with multi-arch binary builds
- Updated GitHub Actions to v6 checkout

## [1.0.0] - 2025-04-01

### Added
- Initial release of `catalog-bundle-inspect`
- Pure Go OLM catalog bundle inspector
- FBC (File-Based Catalog) rendering and resolution
- Channel head resolution with semver support
- Cross-platform builds (linux/amd64, linux/arm64)

[Unreleased]: https://github.com/midu16/opm-troubleshooting/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/midu16/opm-troubleshooting/compare/v0.1...v1.1.0
[1.0.0]: https://github.com/midu16/opm-troubleshooting/releases/tag/v0.1
