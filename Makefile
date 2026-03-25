.PHONY: build run unit-test test fmt fmt-check vet lint ci tidy prek-all prek-install

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
