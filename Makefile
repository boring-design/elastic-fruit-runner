.PHONY: build run unit-test test fmt fmt-check vet lint check ci generate proto-gen tidy prek-all prek-install help

.DEFAULT_GOAL := help

# Build the CLI binary
build:
	@mkdir -p output
	go build -o output/elastic-fruit-runner ./cmd/elastic-fruit-runner/

# Run unit tests
unit-test:
	go test ./...

# Run all tests
test: unit-test

# Format Go code
fmt:
	gofmt -l -w .

# Check formatting without modifying files (fails if unformatted)
fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Files not formatted:"; gofmt -l .; exit 1)

# Run go vet
vet:
	go vet ./...

# Run golangci-lint
lint:
	golangci-lint run

# Run quick local checks before committing (format, vet, build)
check: fmt vet build

# Run all CI checks (same as pre-commit)
ci: fmt-check vet build lint unit-test prek-all

# Run all code generation
generate: proto-gen

# Generate protobuf and Connect RPC code
proto-gen:
	buf generate

# Tidy go modules
tidy:
	go mod tidy

# Run prek on all files
prek-all:
	prek run --all-files

# Install prek git hooks
prek-install:
	prek install

# Show available targets
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Development:"
	@echo "  build       Build the CLI binary to output/"
	@echo "  fmt         Format Go code"
	@echo "  vet         Run go vet"
	@echo "  lint        Run golangci-lint"
	@echo "  check       Run fmt + vet + build (quick local check)"
	@echo "  tidy        Tidy go modules"
	@echo "  generate    Run all code generation"
	@echo "  proto-gen   Generate protobuf and Connect RPC code"
	@echo ""
	@echo "Testing:"
	@echo "  test        Run all tests"
	@echo "  unit-test   Run unit tests"
	@echo ""
	@echo "CI:"
	@echo "  ci          Run all CI checks (fmt-check + vet + build + lint + unit-test)"
	@echo "  fmt-check   Check formatting without modifying files"
	@echo ""
	@echo "Hooks:"
	@echo "  prek-install  Install prek git hooks"
	@echo "  prek-all      Run prek on all files"
