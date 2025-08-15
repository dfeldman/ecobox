#!/bin/bash

# Network Dashboard Build Script

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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

# Check if Node.js is installed
check_node() {
    if ! command -v node &> /dev/null; then
        echo -e "${RED}Error: Node.js is not installed or not in PATH${NC}"
        echo "Please install Node.js from https://nodejs.org/"
        exit 1
    fi
    
    NODE_VERSION=$(node --version)
    echo -e "${GREEN}Found Node.js: ${NODE_VERSION}${NC}"
    
    if ! command -v npm &> /dev/null; then
        echo -e "${RED}Error: npm is not installed or not in PATH${NC}"
        exit 1
    fi
    
    NPM_VERSION=$(npm --version)
    echo -e "${GREEN}Found npm: ${NPM_VERSION}${NC}"
}

# Build frontend
build_frontend() {
    echo -e "${BLUE}Building Vue.js frontend...${NC}"
    
    cd frontend
    
    # Install dependencies if node_modules doesn't exist
    if [ ! -d "node_modules" ]; then
        echo -e "${YELLOW}Installing frontend dependencies...${NC}"
        npm install
    fi
    
    # Build the frontend
    npm run build
    
    cd ..
    
    echo -e "${GREEN}Frontend built successfully to web/static-vue/${NC}"
}

# Build the application
build() {
    echo -e "${BLUE}Building Network Dashboard...${NC}"
    
    # Create output directory
    mkdir -p bin
    
    # Build for current platform
    go build -o bin/dashboard ./cmd/dashboard
    
    echo -e "${GREEN}Backend build complete: bin/dashboard${NC}"
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

# Install frontend dependencies
install_frontend() {
    echo -e "${YELLOW}Installing frontend dependencies...${NC}"
    cd frontend && npm install && cd ..
}

# Clean build artifacts
clean() {
    echo -e "${YELLOW}Cleaning build artifacts...${NC}"
    rm -rf bin/
    rm -rf web/static-vue/
    if [ -d "frontend/node_modules" ]; then
        rm -rf frontend/node_modules
    fi
    if [ -d "frontend/dist" ]; then
        rm -rf frontend/dist
    fi
    go clean
}

# Run the application
run() {
    echo -e "${YELLOW}Starting Network Dashboard...${NC}"
    ./bin/dashboard -config config.toml
}

# Development server with auto-reload
dev() {
    echo -e "${YELLOW}Starting backend development server...${NC}"
    
    # Install air for hot reloading if not present
    if ! command -v air &> /dev/null; then
        echo "Installing air for hot reloading..."
        go install github.com/cosmtrek/air@latest
    fi
    
    air
}

# Development frontend server
dev_frontend() {
    echo -e "${YELLOW}Starting frontend development server...${NC}"
    cd frontend && npm run dev
}

# Show help
help() {
    echo "Network Dashboard Build Script"
    echo ""
    echo "Usage: ./build.sh <command>"
    echo ""
    echo "Commands:"
    echo "  deps              Install Go dependencies"
    echo "  install-frontend  Install frontend dependencies"
    echo "  build-frontend    Build Vue.js frontend"
    echo "  build             Build complete application (backend + frontend)"
    echo "  test              Run tests"
    echo "  run               Run the application"
    echo "  dev               Run backend in development mode with hot reload"
    echo "  dev-frontend      Run frontend development server"
    echo "  clean             Clean all build artifacts"
    echo "  help              Show this help message"
}

# Main script logic
main() {
    case "${1:-help}" in
        deps)
            check_go
            deps
            ;;
        install-frontend)
            check_node
            install_frontend
            ;;
        build-frontend)
            check_node
            build_frontend
            ;;
        build)
            check_go
            check_node
            deps
            build_frontend
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
                check_node
                deps
                build_frontend
                build
            fi
            run
            ;;
        dev)
            check_go
            dev
            ;;
        dev-frontend)
            check_node
            dev_frontend
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
