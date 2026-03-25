.PHONY: build run unit-test test fmt vet lint ci tidy prek-all prek-install

# Build the CLI binary
build:
	go build -o output/elastic-fruit-runner ./cmd/elastic-fruit-runner/

# Run unit tests
unit-test:
	go test ./...

# Run all tests
test: unit-test

# Format Go code
fmt:
	gofmt -l -w .

# Run go vet
vet:
	go vet ./...

# Run golangci-lint
lint:
	golangci-lint run

# Run all CI checks (same as pre-commit)
ci: fmt vet build lint unit-test

# Tidy go modules
tidy:
	go mod tidy

# Run prek on all files
prek-all:
	prek run --all-files

# Install prek git hooks
prek-install:
	prek install
