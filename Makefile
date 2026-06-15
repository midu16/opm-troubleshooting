BINARY := catalog-bundle-inspect
CMD := ./cmd/catalog-bundle-inspect
BIN_DIR := bin
COVERAGE := coverage.out

.PHONY: build test test-functional test-integration test-all lint clean coverage install help

help:
	@echo "Targets: build test test-functional test-integration test-all lint clean coverage install"

build:
	@mkdir -p $(BIN_DIR)
	# Build with containers_image_openpgp tag to avoid gpgme dependency
	# Alternatively, install libgpgme-dev (Ubuntu/Debian) or gpgme-devel (Fedora/RHEL)
	go build -tags containers_image_openpgp -o $(BIN_DIR)/$(BINARY) $(CMD)

test:
	go test -race -cover ./internal/...

test-functional:
	go test -race -cover ./test/functional/...

test-integration:
	go test -tags=integration -race -timeout 20m ./test/integration/...

test-all: test test-functional test-integration

GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint

lint:
	@GOTOOLCHAIN=go1.26.4 command -v $(GOLANGCI_LINT) >/dev/null 2>&1 || GOTOOLCHAIN=go1.26.4 go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	GOTOOLCHAIN=go1.26.4 $(GOLANGCI_LINT) run ./...

clean:
	rm -rf $(BIN_DIR) $(COVERAGE) coverage.html

coverage:
	go test -race -coverprofile=$(COVERAGE) ./internal/... ./test/functional/...
	go tool cover -func=$(COVERAGE)

install:
	go install $(CMD)
