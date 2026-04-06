.PHONY: build build-dashboard run unit-test test fmt fmt-check vet lint check ci tidy prek-all prek-install help

# Build dashboard then Go binary
build: build-dashboard
	@mkdir -p output
	go build -o output/elastic-fruit-runner ./cmd/elastic-fruit-runner/

# Build the React dashboard
build-dashboard:
	cd dashboard && pnpm install --frozen-lockfile && pnpm run build

# Run unit tests only (fast, no external deps)
unit-test: build-dashboard
	go test -short -count=1 ./...

# Run all tests including integration
test: build-dashboard
	go test -count=1 ./...

# Format Go code
fmt:
	gofmt -l -w .

# Check formatting without modifying files (fails if unformatted)
fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Files not formatted:"; gofmt -l .; exit 1)

# Run go vet (requires dashboard/dist/ for embed)
vet: build-dashboard
	go vet ./...

# Run golangci-lint (requires dashboard/dist/ for embed)
lint: build-dashboard
	golangci-lint run

# Run quick local checks before committing (format, vet, build, prek)
check: fmt vet build prek-all

# Run all CI checks (same as pre-commit)
ci: fmt-check vet build lint unit-test

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
	@echo "  build            Build dashboard + Go binary to output/"
	@echo "  build-dashboard  Build React dashboard (required before Go compilation)"
	@echo "  fmt              Format Go code"
	@echo "  vet              Run go vet"
	@echo "  lint             Run golangci-lint"
	@echo "  check            Run fmt + vet + build (quick local check)"
	@echo "  tidy             Tidy go modules"
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
