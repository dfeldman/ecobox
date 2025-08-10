#!/bin/bash

# Network Dashboard Build Script

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if Go is installed
check_go() {
    if ! command -v go &> /dev/null; then
        echo -e "${RED}Error: Go is not installed or not in PATH${NC}"
        echo "Please install Go from https://golang.org/dl/"
        exit 1
    fi
    
    GO_VERSION=$(go version | cut -d' ' -f3)
    echo -e "${GREEN}Found Go: ${GO_VERSION}${NC}"
}

# Build the application
build() {
    echo -e "${YELLOW}Building Network Dashboard...${NC}"
    
    # Create output directory
    mkdir -p bin
    
    # Build for current platform
    go build -o bin/dashboard ./cmd/dashboard
    
    echo -e "${GREEN}Build complete: bin/dashboard${NC}"
}

# Run tests
test() {
    echo -e "${YELLOW}Running tests...${NC}"
    go test ./...
}

# Install dependencies
deps() {
    echo -e "${YELLOW}Installing dependencies...${NC}"
    go mod tidy
    go mod download
}

# Clean build artifacts
clean() {
    echo -e "${YELLOW}Cleaning build artifacts...${NC}"
    rm -rf bin/
    go clean
}

# Run the application
run() {
    echo -e "${YELLOW}Starting Network Dashboard...${NC}"
    ./bin/dashboard -config config.toml
}

# Development server with auto-reload
dev() {
    echo -e "${YELLOW}Starting development server...${NC}"
    
    # Install air for hot reloading if not present
    if ! command -v air &> /dev/null; then
        echo "Installing air for hot reloading..."
        go install github.com/cosmtrek/air@latest
    fi
    
    air
}

# Show help
help() {
    echo "Network Dashboard Build Script"
    echo ""
    echo "Usage: ./build.sh <command>"
    echo ""
    echo "Commands:"
    echo "  deps    Install dependencies"
    echo "  build   Build the application"
    echo "  test    Run tests"
    echo "  run     Run the application"
    echo "  dev     Run in development mode with hot reload"
    echo "  clean   Clean build artifacts"
    echo "  help    Show this help message"
}

# Main script logic
main() {
    case "${1:-help}" in
        deps)
            check_go
            deps
            ;;
        build)
            check_go
            deps
            build
            ;;
        test)
            check_go
            test
            ;;
        run)
            if [ ! -f "bin/dashboard" ]; then
                echo -e "${YELLOW}Binary not found, building first...${NC}"
                check_go
                deps
                build
            fi
            run
            ;;
        dev)
            check_go
            dev
            ;;
        clean)
            clean
            ;;
        help|*)
            help
            ;;
    esac
}

main "$@"
