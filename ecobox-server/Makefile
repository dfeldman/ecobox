# Network Dashboard Makefile

.PHONY: deps build test run dev clean help build-frontend dev-frontend install-frontend

# Default target
all: build

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod tidy
	go mod download

# Install frontend dependencies
install-frontend:
	@echo "Installing frontend dependencies..."
	cd frontend && npm install

# Build frontend
build-frontend: install-frontend
	@echo "Building Vue.js frontend..."
	cd frontend && npm run build
	@echo "Frontend built to web/static-vue/"

# Development frontend server
dev-frontend: install-frontend
	@echo "Starting Vue.js development server..."
	cd frontend && npm run dev

# Build the application
build: deps build-frontend
	@echo "Building Network Dashboard..."
	@mkdir -p bin
	go build -o bin/dashboard ./cmd/dashboard

# Build for multiple platforms
build-all: deps build-frontend
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

# Development with frontend hot reload
dev-full: dev-frontend
	@echo "Run 'make dev' in another terminal for backend hot reload"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -rf web/static-vue/
	cd frontend && rm -rf node_modules dist
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
	@echo "  build           - Build the complete application (backend + frontend)"
	@echo "  build-frontend  - Build only the Vue.js frontend"
	@echo "  install-frontend- Install frontend dependencies"
	@echo "  dev-frontend    - Start Vue.js development server"
	@echo "  dev             - Start backend development server with hot reload"
	@echo "  dev-full        - Start frontend development server (run 'make dev' separately)"
	@echo "  run             - Run the built application"
	@echo "  test            - Run tests"
	@echo "  clean           - Clean build artifacts"
	@echo "  fmt             - Format code"
	@echo "  lint            - Lint code"
	@echo "  release         - Create release archives"
	@echo "  install         - Install to GOPATH/bin"
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
