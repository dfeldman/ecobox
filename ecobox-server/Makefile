# Network Dashboard Makefile

.PHONY: deps build test run dev clean help

# Default target
all: build

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod tidy
	go mod download

# Build the application
build: deps
	@echo "Building Network Dashboard..."
	@mkdir -p bin
	go build -o bin/dashboard ./cmd/dashboard

# Build for multiple platforms
build-all: deps
	@echo "Building for multiple platforms..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 go build -o bin/dashboard-linux-amd64 ./cmd/dashboard
	GOOS=darwin GOARCH=amd64 go build -o bin/dashboard-darwin-amd64 ./cmd/dashboard
	GOOS=darwin GOARCH=arm64 go build -o bin/dashboard-darwin-arm64 ./cmd/dashboard
	GOOS=windows GOARCH=amd64 go build -o bin/dashboard-windows-amd64.exe ./cmd/dashboard

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run the application
run: build
	@echo "Starting Network Dashboard..."
	./bin/dashboard -config config.toml

# Development mode with hot reload (requires air)
dev:
	@echo "Starting development server..."
	@which air > /dev/null || (echo "Installing air..." && go install github.com/cosmtrek/air@latest)
	air

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	go clean

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Lint code (requires golangci-lint)
lint:
	@echo "Linting code..."
	@which golangci-lint > /dev/null || (echo "Please install golangci-lint" && exit 1)
	golangci-lint run

# Generate documentation
docs:
	@echo "Generating documentation..."
	go doc -all ./... > docs/API.md

# Create release archive
release: build-all
	@echo "Creating release archives..."
	@mkdir -p release
	tar -czf release/ecobox-server-linux-amd64.tar.gz -C bin dashboard-linux-amd64 -C .. config.toml README.md
	tar -czf release/ecobox-server-darwin-amd64.tar.gz -C bin dashboard-darwin-amd64 -C .. config.toml README.md
	tar -czf release/ecobox-server-darwin-arm64.tar.gz -C bin dashboard-darwin-arm64 -C .. config.toml README.md
	zip -j release/ecobox-server-windows-amd64.zip bin/dashboard-windows-amd64.exe config.toml README.md

# Install the application to $GOPATH/bin
install: deps
	@echo "Installing Network Dashboard..."
	go install ./cmd/dashboard

# Show help
help:
	@echo "Network Dashboard Makefile"
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  deps       Install dependencies"
	@echo "  build      Build the application"
	@echo "  build-all  Build for multiple platforms"
	@echo "  test       Run tests"
	@echo "  run        Run the application"
	@echo "  dev        Run in development mode with hot reload"
	@echo "  clean      Clean build artifacts"
	@echo "  fmt        Format code"
	@echo "  lint       Lint code (requires golangci-lint)"
	@echo "  docs       Generate documentation"
	@echo "  release    Create release archives"
	@echo "  install    Install to GOPATH/bin"
	@echo "  help       Show this help message"
