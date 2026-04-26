.PHONY: build build-dashboard run unit-test test integration-test fmt fmt-check vet lint check ci tidy prek-all prek-install help

# Build dashboard then Go binary
# `-B gobuildid` derives a Mach-O LC_UUID load command from the Go build ID
# on darwin builds. macOS 15+ Local Network Privacy keys off LC_UUID when
# deciding whether a launchd-spawned binary can reach private subnets such as
# the Tart bridge (192.168.64.x). Without it, brew services daemons hit
# "no route to host". See https://github.com/golang/go/issues/68678.
build: build-dashboard
	@mkdir -p output
	go build -ldflags="-B gobuildid" -o output/elastic-fruit-runner ./cmd/elastic-fruit-runner/

# Build the React dashboard
build-dashboard:
	cd dashboard && pnpm install --frozen-lockfile && pnpm run build

# Run unit tests only (fast, no external deps)
unit-test: build-dashboard
	go test -count=1 ./...

# Run integration tests (requires .env.integration-test)
integration-test: build-dashboard
	@test -f .env.integration-test || (echo "ERROR: .env.integration-test not found."; exit 1)
	set -a && . ./.env.integration-test && set +a && go test -tags=integration -v -count=1 -timeout=15m -coverpkg=./... -coverprofile=coverage-integration.out ./test/integration/

# Run all tests (unit + integration)
test: unit-test integration-test

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

# Run all checks
check: fmt-check vet build lint prek-all unit-test

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
	@echo "  integration-test  Run integration tests (requires EFR_TEST_CONFIG_URL)"
	@echo ""
	@echo "CI:"
	@echo "  ci          Run all CI checks (fmt-check + vet + build + lint + unit-test)"
	@echo "  fmt-check   Check formatting without modifying files"
	@echo ""
	@echo "Hooks:"
	@echo "  prek-install  Install prek git hooks"
	@echo "  prek-all      Run prek on all files"
