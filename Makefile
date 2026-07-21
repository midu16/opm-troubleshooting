# opm-troubleshooting Makefile
# Comprehensive build system for OLM operator troubleshooting tools
#
# Usage: make <target>
#   Run 'make help' for all available targets and detailed descriptions

# ============================================================================
# Configuration
# ============================================================================

# Project metadata
PROJECT_NAME := opm-troubleshooting
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")

# Build configuration
BIN_DIR := bin
COVERAGE_DIR := coverage
COVERAGE_PROFILE := $(COVERAGE_DIR)/coverage.out
COVERAGE_HTML := $(COVERAGE_DIR)/coverage.html

# Go build flags
GO_BUILD_FLAGS := -tags containers_image_openpgp
GO_LDFLAGS := -X main.Version=$(VERSION) \
              -X main.BuildDate=$(BUILD_DATE) \
              -X main.GitCommit=$(GIT_COMMIT) \
              -X main.GitBranch=$(GIT_BRANCH)

# Test configuration
TEST_TIMEOUT := 10m
INTEGRATION_TEST_TIMEOUT := 20m

# Binaries to build
BINARIES := catalog-bundle-inspect \
            batch-validate \
            telco-diagnose \
            opm-diagnose

# Ensure all binaries output to $(BIN_DIR)/
export BIN_DIR

# Tools
GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint
GOFUMPT := $(shell go env GOPATH)/bin/gofumpt

# Colors for output
COLOR_RESET := \033[0m
COLOR_BOLD := \033[1m
COLOR_GREEN := \033[32m
COLOR_YELLOW := \033[33m
COLOR_BLUE := \033[34m
COLOR_CYAN := \033[36m

# ============================================================================
# Default Target
# ============================================================================

.DEFAULT_GOAL := help

# ============================================================================
# Help Target (GitHub Best Practice)
# ============================================================================

.PHONY: help
help: ## Display this help message (default target)
	@echo "$(COLOR_BOLD)$(PROJECT_NAME) - Makefile targets$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_CYAN)Usage:$(COLOR_RESET)"
	@echo "  make $(COLOR_GREEN)<target>$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_CYAN)Build Targets:$(COLOR_RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; /^build/ || /^all$$/ || /^install/ || /^clean/ {printf "  $(COLOR_GREEN)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(COLOR_CYAN)Test Targets:$(COLOR_RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; /test/ {printf "  $(COLOR_GREEN)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(COLOR_CYAN)Quality Targets:$(COLOR_RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; /^lint/ || /^fmt/ || /^vet/ || /^coverage/ {printf "  $(COLOR_GREEN)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(COLOR_CYAN)Development Targets:$(COLOR_RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; /^dev/ || /^watch/ || /^mod/ {printf "  $(COLOR_GREEN)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(COLOR_CYAN)CI/CD Targets:$(COLOR_RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; /^ci/ {printf "  $(COLOR_GREEN)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(COLOR_CYAN)Information:$(COLOR_RESET)"
	@echo "  Version:     $(VERSION)"
	@echo "  Commit:      $(GIT_COMMIT)"
	@echo "  Branch:      $(GIT_BRANCH)"
	@echo "  Build Dir:   $(BIN_DIR)/"
	@echo ""

# ============================================================================
# Build Targets
# ============================================================================

.PHONY: all
all: clean build ## Build all binaries (clean + build)

.PHONY: build
build: $(BINARIES) ## Build all binaries to bin/ directory

.PHONY: catalog-bundle-inspect
catalog-bundle-inspect: ## Build catalog-bundle-inspect binary
	@echo "$(COLOR_BLUE)Building catalog-bundle-inspect...$(COLOR_RESET)"
	@mkdir -p $(BIN_DIR)
	@go build $(GO_BUILD_FLAGS) -ldflags "$(GO_LDFLAGS)" -o $(BIN_DIR)/$@ ./cmd/$@
	@echo "$(COLOR_GREEN)✓ Built $(BIN_DIR)/$@$(COLOR_RESET)"

.PHONY: batch-validate
batch-validate: ## Build batch-validate binary
	@echo "$(COLOR_BLUE)Building batch-validate...$(COLOR_RESET)"
	@mkdir -p $(BIN_DIR)
	@go build $(GO_BUILD_FLAGS) -ldflags "$(GO_LDFLAGS)" -o $(BIN_DIR)/$@ ./cmd/$@
	@echo "$(COLOR_GREEN)✓ Built $(BIN_DIR)/$@$(COLOR_RESET)"

.PHONY: telco-diagnose
telco-diagnose: ## Build telco-diagnose binary
	@echo "$(COLOR_BLUE)Building telco-diagnose...$(COLOR_RESET)"
	@mkdir -p $(BIN_DIR)
	@go build $(GO_BUILD_FLAGS) -ldflags "$(GO_LDFLAGS)" -o $(BIN_DIR)/$@ ./cmd/$@
	@echo "$(COLOR_GREEN)✓ Built $(BIN_DIR)/$@$(COLOR_RESET)"

.PHONY: opm-diagnose
opm-diagnose: ## Build opm-diagnose unified diagnostic binary
	@echo "$(COLOR_BLUE)Building opm-diagnose...$(COLOR_RESET)"
	@mkdir -p $(BIN_DIR)
	@go build $(GO_BUILD_FLAGS) -ldflags "$(GO_LDFLAGS)" -o $(BIN_DIR)/$@ ./cmd/$@
	@echo "$(COLOR_GREEN)✓ Built $(BIN_DIR)/$@$(COLOR_RESET)"

.PHONY: install
install: build ## Install binaries to $GOPATH/bin
	@echo "$(COLOR_BLUE)Installing binaries to GOPATH...$(COLOR_RESET)"
	@for bin in $(BINARIES); do \
		cp $(BIN_DIR)/$$bin $(shell go env GOPATH)/bin/; \
		echo "$(COLOR_GREEN)✓ Installed $$bin$(COLOR_RESET)"; \
	done

# ============================================================================
# Test Targets
# ============================================================================

.PHONY: test
test: ## Run unit tests for internal packages
	@echo "$(COLOR_BLUE)Running unit tests...$(COLOR_RESET)"
	@go test -race -cover -timeout $(TEST_TIMEOUT) ./internal/...
	@echo "$(COLOR_GREEN)✓ Unit tests passed$(COLOR_RESET)"

.PHONY: test-functional
test-functional: ## Run functional tests with mocks
	@echo "$(COLOR_BLUE)Running functional tests...$(COLOR_RESET)"
	@go test -race -cover -timeout $(TEST_TIMEOUT) ./test/functional/...
	@echo "$(COLOR_GREEN)✓ Functional tests passed$(COLOR_RESET)"

.PHONY: test-integration
test-integration: ## Run integration tests (requires network)
	@echo "$(COLOR_BLUE)Running integration tests...$(COLOR_RESET)"
	@go test -tags=integration -race -timeout $(INTEGRATION_TEST_TIMEOUT) ./test/integration/...
	@echo "$(COLOR_GREEN)✓ Integration tests passed$(COLOR_RESET)"

.PHONY: test-all
test-all: test test-functional test-integration ## Run all tests (unit, functional, integration)

.PHONY: test-verbose
test-verbose: ## Run all tests with verbose output
	@echo "$(COLOR_BLUE)Running all tests (verbose)...$(COLOR_RESET)"
	@go test -v -race -cover -timeout $(TEST_TIMEOUT) ./internal/... ./test/functional/...
	@go test -v -tags=integration -race -timeout $(INTEGRATION_TEST_TIMEOUT) ./test/integration/...

.PHONY: test-must-gather
test-must-gather: ## Run must-gather analysis tests
	@echo "$(COLOR_BLUE)Running must-gather analysis tests...$(COLOR_RESET)"
	@go test -v -race -timeout $(TEST_TIMEOUT) ./internal/mustgather/... ./internal/analysis/...
	@echo "$(COLOR_GREEN)✓ Must-gather tests passed$(COLOR_RESET)"

# ============================================================================
# Quality Targets
# ============================================================================

.PHONY: lint
lint: ## Run golangci-lint on all packages
	@echo "$(COLOR_BLUE)Running linters...$(COLOR_RESET)"
	@command -v $(GOLANGCI_LINT) >/dev/null 2>&1 || { \
		echo "$(COLOR_YELLOW)Installing golangci-lint...$(COLOR_RESET)"; \
		GOTOOLCHAIN=go1.26.4 go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	}
	@GOTOOLCHAIN=go1.26.4 $(GOLANGCI_LINT) run ./...
	@echo "$(COLOR_GREEN)✓ Linting passed$(COLOR_RESET)"

.PHONY: fmt
fmt: ## Format Go code with gofumpt
	@echo "$(COLOR_BLUE)Formatting code...$(COLOR_RESET)"
	@command -v $(GOFUMPT) >/dev/null 2>&1 || { \
		echo "$(COLOR_YELLOW)Installing gofumpt...$(COLOR_RESET)"; \
		go install mvdan.cc/gofumpt@latest; \
	}
	@$(GOFUMPT) -l -w .
	@echo "$(COLOR_GREEN)✓ Code formatted$(COLOR_RESET)"

.PHONY: vet
vet: ## Run go vet on all packages
	@echo "$(COLOR_BLUE)Running go vet...$(COLOR_RESET)"
	@go vet ./...
	@echo "$(COLOR_GREEN)✓ Vet passed$(COLOR_RESET)"

.PHONY: coverage
coverage: ## Generate test coverage report
	@echo "$(COLOR_BLUE)Generating coverage report...$(COLOR_RESET)"
	@mkdir -p $(COVERAGE_DIR)
	@go test -race -coverprofile=$(COVERAGE_PROFILE) -covermode=atomic \
		./internal/... ./test/functional/...
	@go tool cover -func=$(COVERAGE_PROFILE)
	@go tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML)
	@echo "$(COLOR_GREEN)✓ Coverage report generated: $(COVERAGE_HTML)$(COLOR_RESET)"

.PHONY: coverage-view
coverage-view: coverage ## Generate and view coverage report in browser
	@echo "$(COLOR_BLUE)Opening coverage report...$(COLOR_RESET)"
	@xdg-open $(COVERAGE_HTML) 2>/dev/null || open $(COVERAGE_HTML) 2>/dev/null || \
		echo "$(COLOR_YELLOW)Coverage report at: $(COVERAGE_HTML)$(COLOR_RESET)"

# ============================================================================
# Development Targets
# ============================================================================

.PHONY: dev
dev: clean fmt vet lint test build ## Development workflow (format, vet, lint, test, build)

.PHONY: mod-tidy
mod-tidy: ## Run go mod tidy
	@echo "$(COLOR_BLUE)Tidying go modules...$(COLOR_RESET)"
	@go mod tidy
	@echo "$(COLOR_GREEN)✓ Modules tidied$(COLOR_RESET)"

.PHONY: mod-download
mod-download: ## Download go modules
	@echo "$(COLOR_BLUE)Downloading go modules...$(COLOR_RESET)"
	@go mod download
	@echo "$(COLOR_GREEN)✓ Modules downloaded$(COLOR_RESET)"

.PHONY: mod-verify
mod-verify: ## Verify go modules
	@echo "$(COLOR_BLUE)Verifying go modules...$(COLOR_RESET)"
	@go mod verify
	@echo "$(COLOR_GREEN)✓ Modules verified$(COLOR_RESET)"

.PHONY: deps
deps: mod-download mod-verify ## Download and verify dependencies

# ============================================================================
# CI/CD Targets
# ============================================================================

.PHONY: ci
ci: deps lint vet test-all build ## CI pipeline (deps, lint, vet, test-all, build)

.PHONY: ci-quick
ci-quick: lint test build ## Quick CI check (lint, test, build)

# ============================================================================
# Clean Targets
# ============================================================================

.PHONY: clean
clean: ## Remove build artifacts and coverage reports
	@echo "$(COLOR_BLUE)Cleaning build artifacts...$(COLOR_RESET)"
	@rm -rf $(BIN_DIR) $(COVERAGE_DIR)
	@echo "$(COLOR_GREEN)✓ Clean complete$(COLOR_RESET)"

.PHONY: clean-all
clean-all: clean ## Remove all generated files including caches
	@echo "$(COLOR_BLUE)Cleaning all generated files...$(COLOR_RESET)"
	@go clean -cache -testcache -modcache
	@echo "$(COLOR_GREEN)✓ Deep clean complete$(COLOR_RESET)"

# ============================================================================
# Utility Targets
# ============================================================================

.PHONY: version
version: ## Display version information
	@echo "Version:     $(VERSION)"
	@echo "Build Date:  $(BUILD_DATE)"
	@echo "Git Commit:  $(GIT_COMMIT)"
	@echo "Git Branch:  $(GIT_BRANCH)"
	@echo "Go Version:  $(shell go version)"

.PHONY: list-binaries
list-binaries: ## List all available binaries
	@echo "$(COLOR_CYAN)Available binaries:$(COLOR_RESET)"
	@for bin in $(BINARIES); do \
		echo "  - $$bin"; \
	done

.PHONY: check-env
check-env: ## Check development environment
	@echo "$(COLOR_CYAN)Environment Check:$(COLOR_RESET)"
	@echo "  Go version:       $(shell go version)"
	@echo "  GOPATH:           $(shell go env GOPATH)"
	@echo "  GOROOT:           $(shell go env GOROOT)"
	@echo "  Build dir:        $(BIN_DIR)/"
	@echo "  golangci-lint:    $(shell command -v $(GOLANGCI_LINT) >/dev/null 2>&1 && echo "installed" || echo "not found")"
	@echo "  gofumpt:          $(shell command -v $(GOFUMPT) >/dev/null 2>&1 && echo "installed" || echo "not found")"

# ============================================================================
# Phony Target Declaration
# ============================================================================

.PHONY: help all build install test test-functional test-integration test-all \
        test-verbose test-must-gather lint fmt vet coverage coverage-view dev \
        mod-tidy mod-download mod-verify deps ci ci-quick clean clean-all \
        version list-binaries check-env \
        catalog-bundle-inspect batch-validate telco-diagnose opm-diagnose
