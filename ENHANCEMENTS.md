# OPM Troubleshooting Enhancements

## Overview

This document summarizes the enhancements made to the opm-troubleshooting project to support comprehensive operator fault analysis with AI-powered root cause detection.

**Version:** v0.1 (dev branch)  
**Enhancement Date:** 2026-06-17

---

## Major Enhancements

### 1. Must-Gather Fault Analysis

**New Capability:** AI-powered analysis of OpenShift must-gather dumps to identify faulty operators and provide actionable recommendations.

#### Features

- **Automatic Operator Discovery**: Parses OLM resources (Subscriptions, CSVs) from must-gather directory structure
- **Fault Detection**: Identifies operators with issues based on subscription state and conditions
- **Version Delta Analysis**: Compares installed versions with target versions from catalog
- **Git Commit Tracking**: Resolves build commits for operators from bundle metadata
- **Claude API Integration**: Uses AI to correlate code changes with failure symptoms

#### New Packages

**`internal/mustgather/`** - Must-gather parsing and fault detection
- `ParseMustGather()`: Main entry point for directory parsing
- `OperatorState`: Captures runtime state from Subscriptions and CSVs
- `isFaulty()`: Detects faulty operators based on OLM conditions
- Test coverage: 20.9%

**`internal/gitdelta/`** - Git commit delta calculator
- `CalculateDelta()`: Clones repos and computes diffs between commits
- Shallow cloning for efficiency
- Diff statistics (files changed, additions, deletions)

**`internal/claudeapi/`** - Claude API integration
- `NewClient()`: Creates authenticated Claude API client
- `AnalyzeFault()`: Sends fault analysis requests
- Structured prompt templates for operator troubleshooting
- Response parsing for actionable insights

**`internal/analysis/`** - Fault analysis orchestration
- `AnalyzeMustGather()`: Main workflow coordinator
- `FaultReport`: Complete analysis results per operator
- `AnalysisResult`: Aggregated results for all operators
- Non-fatal error handling for graceful degradation

#### Usage Example

```bash
export ANTHROPIC_API_KEY=sk-ant-...
export DOCKER_CONFIG=~/.docker

./bin/catalog-bundle-inspect \
  --must-gather /path/to/must-gather.local.123456 \
  --package kubernetes-nmstate-operator \
  --catalog quay.io/prega/prega-operator-index:v4.22-latest \
  --json > analysis.json
```

---

### 2. RCA Pattern Detection

**New Capability:** Root cause analysis pattern detection based on OpenShift RCA best practices.

#### Features

Implements 7 RCA patterns from world-class OpenShift RCAs:

1. **ASYMMETRY** - Different behavior in similar contexts
2. **MISSING_GUARD** - Missing null/error checks
3. **TYPE_ESCALATION** - Error type changes breaking handling
4. **STATE_DIVERGENCE** - Inconsistent state across components
5. **DEFAULT_INVERSION** - Default behavior changed between versions
6. **RACE_CONDITION** - Timing-dependent failures
7. **ERROR_SWALLOWING** - Errors silently ignored

#### Implementation

**`internal/rca/patterns.go`**
- `PatternDetector`: Rule-based pattern detection engine
- `DetectPatterns()`: Analyzes failure symptoms
- `GetRecommendations()`: Provides actionable fix recommendations
- Test coverage: 94.4%

#### Pattern Detection Example

```go
detector := rca.NewPatternDetector()
patterns := detector.DetectPatterns("panic: nil pointer dereference")
// Returns: PatternMissingGuard with confidence 0.8
```

#### Recommendations

Each pattern provides priority-ranked fix recommendations:

**MISSING_GUARD Example:**
1. [Critical] Add nil/null guard checks
2. [High] Use safe navigation operators

**RACE_CONDITION Example:**
1. [Critical] Add synchronization primitives (mutex/lock)
2. [Critical] Add startup guard flag
3. [High] Introduce message queue

---

### 3. Enhanced Makefile (GitHub Best Practices)

**New Capability:** Comprehensive build system following GitHub's Makefile best practices.

#### Features

- **Colored Output**: Color-coded help and build messages
- **Categorized Help Menu**: Organized by Build, Test, Quality, Development, CI/CD
- **Version Information**: Git version, commit, branch tracking
- **All Binaries to bin/**: Ensures consistent build output location
- **Multiple Test Targets**: unit, functional, integration, must-gather specific
- **Quality Targets**: lint, fmt, vet, coverage
- **CI/CD Targets**: Pre-configured pipelines for automation

#### Help Menu Structure

```
$ make help

opm-troubleshooting - Makefile targets

Usage:
  make <target>

Build Targets:
  build                Build all binaries to bin/ directory
  install              Install binaries to $GOPATH/bin
  clean                Remove build artifacts

Test Targets:
  test                 Run unit tests
  test-functional      Run functional tests
  test-integration     Run integration tests
  test-all             Run all tests
  test-must-gather     Run must-gather analysis tests

Quality Targets:
  lint                 Run golangci-lint
  fmt                  Format code with gofumpt
  vet                  Run go vet
  coverage             Generate coverage report
  coverage-view        View coverage in browser

Development Targets:
  dev                  Development workflow (fmt, vet, lint, test, build)
  mod-tidy             Tidy go modules
  deps                 Download and verify dependencies

CI/CD Targets:
  ci                   Full CI pipeline
  ci-quick             Quick CI check
```

#### Build System Features

**Consistent Binary Location:**
- All binaries built to `bin/` directory
- Never pollutes project root or subdirectories
- Clean separation of source and artifacts

**Version Embedding:**
```makefile
GO_LDFLAGS := -X main.Version=$(VERSION) \
              -X main.BuildDate=$(BUILD_DATE) \
              -X main.GitCommit=$(GIT_COMMIT)
```

**Development Workflow:**
```bash
make dev    # fmt → vet → lint → test → build
make ci     # deps → lint → vet → test-all → build
```

---

### 4. Comprehensive Test Suite

**Enhancement:** Expanded test coverage for all new packages with real-world test scenarios.

#### Test Coverage

| Package | Coverage | Tests |
|---------|----------|-------|
| rca | 94.4% | Pattern detection, recommendations |
| catalog | 74.9% | Channel resolution, versioning |
| bundlecsv | 75.5% | CSV parsing, repository URLs |
| imageinspect | 64.8% | Bundle metadata extraction |
| workflow | 57.7% | Bundle inspection workflow |
| cli | 25.0% | Command-line interface |
| mustgather | 20.9% | Must-gather parsing |

#### New Test Files

**`internal/rca/patterns_test.go`**
- Pattern detection validation
- Recommendation generation tests
- Evidence collection verification

**`internal/mustgather/parse_test.go`**
- Must-gather directory parsing
- Fault detection logic
- Version extraction
- Real must-gather integration test

#### Test Execution

```bash
# Unit tests only
make test

# All tests
make test-all

# Must-gather specific tests
make test-must-gather

# With coverage report
make coverage
make coverage-view    # Opens HTML report in browser

# Verbose output
make test-verbose
```

#### Integration Test Support

Environment variable for real must-gather testing:
```bash
export TEST_MUST_GATHER_PATH=/path/to/must-gather.local.123456
make test
```

---

## Build System

### Binary Structure

```
bin/
├── catalog-bundle-inspect    # Main CLI tool (65MB)
└── batch-validate            # Batch validation tool (65MB)
```

### Build Commands

```bash
# Build all binaries
make build

# Build specific binary
make catalog-bundle-inspect

# Install to GOPATH
make install

# Clean build artifacts
make clean

# Deep clean (includes caches)
make clean-all
```

### Build Flags

- **containers_image_openpgp** tag: Avoids gpgme dependency
- **Version embedding**: Git version, commit, branch
- **Binary size**: ~65MB per binary (includes operator-registry, go-containerregistry)

---

## Architecture

### Data Flow

```
must-gather directory
    ↓
mustgather.ParseMustGather()
    ↓
analysis.AnalyzeMustGather()
    ├→ catalog.RenderCatalog() (optional)
    ├→ workflow.InspectBundle()
    ├→ gitdelta.CalculateDelta()
    ├→ rca.DetectPatterns()
    └→ claudeapi.AnalyzeFault() (optional)
    ↓
FaultReport
    ├→ Operator state
    ├→ Bundle metadata
    ├→ Git commit delta
    ├→ RCA patterns
    ├→ Recommendations
    └→ Claude AI analysis
    ↓
JSON or human-readable output
```

### Package Dependencies

```
internal/
├── cli/              → analysis, workflow
├── workflow/         → catalog, imageinspect
├── analysis/         → mustgather, catalog, workflow, gitdelta, claudeapi, rca
├── mustgather/       → (standalone)
├── rca/              → (standalone)
├── gitdelta/         → (standalone)
├── claudeapi/        → (standalone)
├── catalog/          → (operator-registry)
└── imageinspect/     → bundlecsv (go-containerregistry)
```

---

## Testing Results

### Real Must-Gather Analysis

**Test Environment:**
- Cluster: OCP 4.16.14
- Must-gather: `/home/midu/04416832/must-gather.local.3030260659186276243`
- Total Operators: 15
- Faulty Operators: 4

**Results:**
```
✓ Parsed 15 operators in 0.48s
✓ Detected 4 faulty operators
✓ Build commits identified: 13/15 (87%)
✓ RCA patterns detected for applicable failures
✓ Recommendations generated
```

**Faulty Operators Detected:**
1. falcon-operator - UpgradePending (CatalogSourcesUnhealthy)
2. cluster-logging - RequirementsNotMet
3. cluster-observability-operator - RequirementsNotMet
4. ocs-client-operator - RequirementsNotMet

---

## Performance

### Build Performance

```
$ time make clean build
real    0m3.821s
user    0m15.293s
sys     0m2.146s
```

### Test Performance

```
$ make test
internal/rca          1.014s  (94.4% coverage)
internal/cli          1.053s  (25.0% coverage)
internal/mustgather   1.013s  (20.9% coverage)
[cached packages]     0.000s
Total: ~3 seconds
```

### Must-Gather Parsing

```
15 operators parsed in 0.48s
Average: 32ms per operator
```

---

## Environment Variables

### Required (for full functionality)

- **ANTHROPIC_API_KEY**: Claude API key for AI analysis
  ```bash
  export ANTHROPIC_API_KEY=sk-ant-api03-...
  ```

- **DOCKER_CONFIG**: Registry authentication
  ```bash
  export DOCKER_CONFIG=~/.docker
  ```

### Optional

- **TEST_MUST_GATHER_PATH**: Real must-gather for integration tests
  ```bash
  export TEST_MUST_GATHER_PATH=/path/to/must-gather.local.123456
  ```

- **REGISTRY_AUTH_FILE**: Alternative registry auth
  ```bash
  export REGISTRY_AUTH_FILE=/path/to/config.json
  ```

---

## Graceful Degradation

The tool works in multiple modes with graceful degradation:

1. **Full Mode** (catalog + must-gather + Claude API)
   - Complete fault analysis with AI recommendations

2. **Catalog Mode** (catalog + must-gather, no Claude API)
   - Bundle metadata + git delta + RCA patterns
   - No AI-generated insights

3. **Must-Gather Only Mode** (no catalog, no Claude API)
   - Operator state parsing
   - Fault detection
   - RCA pattern detection
   - Basic recommendations

4. **Bundle Inspection Mode** (catalog only, original functionality)
   - No must-gather analysis
   - Bundle metadata retrieval only

---

## Future Enhancements

### Potential Improvements

1. **Multi-operator batch analysis**: Analyze all faulty operators in one run
2. **InstallPlan inspection**: Parse InstallPlan resources for deeper context
3. **Event correlation**: Extract and correlate Kubernetes events
4. **Webhook failure detection**: Identify admission webhook issues
5. **Dependency graph visualization**: Show operator dependency chains
6. **Historical trend analysis**: Compare multiple must-gathers over time
7. **Custom RCA pattern plugins**: Allow users to define domain-specific patterns
8. **Automated fix generation**: Generate code patches for common issues

### Test Coverage Goals

- Target 80% coverage for all new packages
- Add end-to-end integration tests
- Add benchmark tests for performance regression detection

---

## Breaking Changes

None. All enhancements are additive and maintain backward compatibility with existing functionality.

---

## Contributors

- Enhanced Makefile following GitHub best practices
- RCA pattern detection based on OpenShift engineering RCAs
- Must-gather analysis integration
- Claude AI-powered fault isolation
- Comprehensive test suite

---

## References

- **OpenShift RCA Best Practices**: `/home/midu/claude-skill/openshift-rca/`
- **OLM Documentation**: https://olm.operatorframework.io/
- **Operator Registry**: https://github.com/operator-framework/operator-registry
- **Go Containerregistry**: https://github.com/google/go-containerregistry
