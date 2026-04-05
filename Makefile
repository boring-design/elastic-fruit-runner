.PHONY: help build build-go build-all clean test fmt fmt-check vet lint check ci generate proto-gen tidy dashboard dashboard-clean prek-all prek-install

# Build variables
BINARY_NAME := elastic-fruit-runner
BUILD_DIR := output
GO := go
GOFLAGS := -v

# Dashboard variables
DASHBOARD_DIR := dashboard

# Version info: tag if tagged, "untagged" otherwise; sha with -dirty suffix if dirty
GIT_SHA := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_DIRTY := $(shell git diff-index --quiet HEAD -- 2>/dev/null || echo "dirty")
GIT_TAG := $(shell git describe --tags --exact-match 2>/dev/null)

ifdef GIT_TAG
    VERSION := $(GIT_TAG)
else
    VERSION := untagged
endif

ifdef GIT_DIRTY
    GIT_SHA := $(GIT_SHA)-dirty
endif

VERSION_PKG := github.com/boring-design/elastic-fruit-runner/internal/controller
LDFLAGS := -s -w -X $(VERSION_PKG).Version=$(VERSION) -X $(VERSION_PKG).CommitSHA=$(GIT_SHA)

help: ## Show this help message
	@echo "Elastic Fruit Runner - Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ { printf "  %-18s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

build: dashboard build-go ## Build binary with dashboard

build-go: ## Build Go binary only
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/elastic-fruit-runner/

build-all: ## Build for all platforms (linux/darwin, amd64/arm64)
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/elastic-fruit-runner/
	GOOS=linux GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/elastic-fruit-runner/
	GOOS=darwin GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/elastic-fruit-runner/
	GOOS=darwin GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/elastic-fruit-runner/

clean: dashboard-clean ## Remove build artifacts
	rm -rf $(BUILD_DIR)
	rm -f coverage.out

test: ## Run tests
	$(GO) test ./...

fmt: ## Format Go code
	gofmt -l -w .

fmt-check: ## Check formatting without modifying files
	@test -z "$$(gofmt -l .)" || (echo "Files not formatted:"; gofmt -l .; exit 1)

vet: ## Run go vet
	$(GO) vet ./...

lint: ## Run golangci-lint
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed, skipping"; \
	fi

check: fmt vet build-go ## Quick local check (fmt + vet + build)

ci: fmt-check vet build-go lint test prek-all ## Run all CI checks

generate: proto-gen ## Run all code generation

proto-gen: ## Generate protobuf and Connect RPC code
	buf generate

tidy: ## Tidy go modules
	$(GO) mod tidy

dashboard: ## Build dashboard
	cd $(DASHBOARD_DIR) && pnpm install && pnpm run build

dashboard-clean: ## Clean dashboard build artifacts
	rm -rf $(DASHBOARD_DIR)/node_modules $(DASHBOARD_DIR)/dist

prek-all: ## Run prek on all files
	prek run --all-files

prek-install: ## Install prek git hooks
	prek install
