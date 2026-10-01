.PHONY: build build-dashboard run unit-test test integration-test fmt fmt-check vet lint sqlc-generate sqlc-check check ci tidy prek-all prek-install help

SQLC_VERSION := v1.30.0

# Build dashboard then Go binary
build: build-dashboard
	@mkdir -p output
	go build -o output/elastic-fruit-runner ./cmd/elastic-fruit-runner/

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

# Regenerate sqlc code from internal/storage/sqlc/queries.sql
sqlc-generate:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate -f internal/storage/sqlc/sqlc.yaml

# Fail when generated sqlc code is out of date
sqlc-check:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) diff -f internal/storage/sqlc/sqlc.yaml

# Run all checks
check: fmt-check vet build lint sqlc-check prek-all unit-test

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
	@echo "  sqlc-generate    Regenerate sqlc code from queries.sql"
	@echo "  sqlc-check       Fail when generated sqlc code is out of date"
	@echo "  check            Run fmt-check + vet + build + lint + sqlc-check + prek + unit-test"
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
