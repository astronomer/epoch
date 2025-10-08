.PHONY: test test-ginkgo test-unit test-examples validate-fmt build clean help coverage deps release-dry-run release-local

# Default target
.DEFAULT_GOAL := help

## test: Run all tests with race detection and coverage
test:
	@echo "Running tests..."
	go test -race -coverprofile=coverage.out -covermode=atomic -v ./...
	@echo "Validating examples compile..."
	go run validate.go

## test-ginkgo: Run tests using Ginkgo (if installed)
test-ginkgo:
	@echo "Running tests with Ginkgo..."
	@if command -v ginkgo >/dev/null 2>&1; then \
		ginkgo -r --race --cover --coverprofile=coverage.out --covermode=atomic -v ./epoch; \
	else \
		echo "Ginkgo not installed, falling back to go test..."; \
		$(MAKE) test; \
	fi

## test-unit: Run only unit tests (epoch package)
test-unit:
	@echo "Running unit tests..."
	go test -race -coverprofile=coverage.out -covermode=atomic -v ./epoch

## test-examples: Validate that examples compile
test-examples:
	@echo "Validating examples compile..."
	go run validate.go

## validate-fmt: Check code formatting and run linters
validate-fmt:
	@echo "Checking code formatting..."
	@output=$$(gofmt -l ./); \
	if [ -n "$$output" ]; then \
		echo "$$output"; \
		echo "Please run 'make fmt' to format the code"; \
		exit 1; \
	fi

## build: Build the project
build:
	@echo "Building..."
	go build -v ./...

## fmt: Format code
fmt:
	@echo "Formatting code..."
	gofmt -w .

## clean: Clean build artifacts and caches
clean:
	@echo "Cleaning..."
	go clean -cache -testcache -modcache

## coverage: Generate and view test coverage report
coverage: test
	@echo "Generating coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod verify

## release-dry-run: Test GoReleaser configuration without releasing
release-dry-run:
	@echo "Testing GoReleaser configuration..."
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser release --snapshot --skip-publish --clean; \
	else \
		echo "GoReleaser not installed. Install with: go install github.com/goreleaser/goreleaser@latest"; \
		exit 1; \
	fi

## release-local: Build release locally (snapshot)
release-local:
	@echo "Building local release..."
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser build --snapshot --clean; \
	else \
		echo "GoReleaser not installed. Install with: go install github.com/goreleaser/goreleaser@latest"; \
		exit 1; \
	fi

## help: Show this help message
help:
	@echo "Available targets:"
	@grep -E '^##' $(MAKEFILE_LIST) | sed 's/##//g' | sort
