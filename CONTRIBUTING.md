# Contributing to opm-troubleshooting

Thank you for your interest in contributing to opm-troubleshooting! This document provides guidelines and instructions for contributing.

## Code of Conduct

By participating in this project, you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).

## How to Contribute

### Reporting Bugs

Before creating a bug report, please check the [existing issues](https://github.com/midu16/opm-troubleshooting/issues) to avoid duplicates. When creating a bug report, use the **Bug Report** issue template and include as much detail as possible.

### Suggesting Features

Feature requests are welcome. Use the **Feature Request** issue template and describe the problem you're trying to solve and how you envision the solution.

### Pull Requests

1. Fork the repository
2. Create a feature branch from `main`:
   ```bash
   git checkout -b feat/your-feature main
   ```
3. Make your changes following the [development guidelines](#development-guidelines)
4. Commit your changes using [conventional commits](#commit-messages)
5. Push to your fork and open a Pull Request against `main`

## Development Guidelines

### Prerequisites

- Go 1.26+ (version specified in `go.mod`)
- `make`
- `golangci-lint`

### Building

```bash
make build
```

### Running Tests

```bash
# Unit tests
make test

# Functional tests
make test-functional

# All tests with race detection
make test-all

# Coverage report
make coverage
```

### Linting

```bash
make lint
```

### Formatting

```bash
make fmt
```

### Full CI check locally

```bash
make ci
```

## Commit Messages

This project follows [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`.

Examples:
- `feat(catalog): add support for FBC v2 schema`
- `fix(rca): correct false positive in CrashLoopBackOff detection`
- `docs: update installation instructions`

## Project Structure

```
cmd/                    # CLI entry points
internal/               # Private packages
  catalog/              # FBC rendering and resolution
  cli/                  # CLI handlers
  rca/                  # Root cause analysis
  mustgather/           # Must-gather parsing
  healthcheck/          # Health checker
  ...
test/
  functional/           # Functional tests
  integration/          # Integration tests
testdata/               # Test fixtures
```

## Code Review

All submissions require review before merging. Reviewers will look for:

- Correctness and test coverage
- Adherence to existing code patterns
- Clean `golangci-lint` output
- No unnecessary dependencies

## License

By contributing, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE).
