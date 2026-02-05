.PHONY: all build test lint fmt vet clean coverage help

# Default target
all: fmt vet lint test build

# Build the application
build:
	@echo "Building..."
	go build -o bin/mcp-server ./cmd/server/main.go

# Run tests
test:
	@echo "Running tests..."
	go test -v -race ./...

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	gofmt -w .

# Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

# Security scan
security:
	@echo "Running security scan..."
	gosec ./...

# Check dependencies for vulnerabilities
audit:
	@echo "Auditing dependencies..."
	nancy sleuth

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# Run all checks (CI pipeline locally)
ci: fmt vet lint test build

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod verify

# Run integration tests (requires database)
integration-test:
	@echo "Running integration tests..."
	@echo "Make sure PostgreSQL and MySQL are running"
	go test -v ./internal/mcp -run TestLive -count=1

# Development server
dev:
	@echo "Starting development server..."
	go run ./cmd/server/main.go

# Help
help:
	@echo "Available targets:"
	@echo "  make build          - Build the application"
	@echo "  make test           - Run unit tests"
	@echo "  make coverage       - Run tests with coverage report"
	@echo "  make lint           - Run linter"
	@echo "  make fmt            - Format code"
	@echo "  make vet            - Run go vet"
	@echo "  make security       - Run security scan"
	@echo "  make audit          - Audit dependencies"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make ci             - Run all CI checks locally"
	@echo "  make deps           - Install dependencies"
	@echo "  make integration-test - Run integration tests"
	@echo "  make dev            - Start development server"
	@echo "  make all            - Run fmt, vet, lint, test, build"
