# ABOUTME: Makefile for building and managing the ACP relay server
# ABOUTME: Provides targets for building, testing, and running the relay

.PHONY: build test clean run help

# Build the relay server
build:
	@echo "Building acp-relay..."
	@go build -o acp-relay ./cmd/relay
	@echo "✓ Binary built: ./acp-relay"

# Run tests
test:
	@echo "Running tests..."
	@go test ./... -v

# Run tests without verbose output
test-short:
	@go test ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f acp-relay main relay
	@rm -f *.db *.db-shm *.db-wal
	@echo "✓ Cleaned"

# Run the relay server with default config
run: build
	@./acp-relay --config config.yaml

# Run with codex config
run-codex: build
	@./acp-relay --config codex.yaml

# Install dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@echo "✓ Dependencies downloaded"

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "✓ Code formatted"

# Run linter
lint:
	@echo "Running linter..."
	@golangci-lint run ./... || echo "Note: Install golangci-lint for linting"

# Show help
help:
	@echo "ACP Relay - Available targets:"
	@echo ""
	@echo "  make build       - Build the acp-relay binary"
	@echo "  make test        - Run all tests (verbose)"
	@echo "  make test-short  - Run all tests (quiet)"
	@echo "  make clean       - Remove build artifacts and databases"
	@echo "  make run         - Build and run with config.yaml"
	@echo "  make run-codex   - Build and run with codex.yaml"
	@echo "  make deps        - Download Go dependencies"
	@echo "  make fmt         - Format Go code"
	@echo "  make lint        - Run linter (requires golangci-lint)"
	@echo "  make help        - Show this help message"
