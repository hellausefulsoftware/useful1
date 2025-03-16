# useful1 - Automated GitHub Issue Resolution Tool
# Makefile for building, testing, and deploying

# Go parameters
GOCMD = go
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOGET = $(GOCMD) get
GOMOD = $(GOCMD) mod
GOVET = $(GOCMD) vet
GOFMT = $(GOCMD) fmt
GOLINT = golint

# Binary name
BINARY_NAME = useful1
BINARY_DIR = bin
BINARY_PATH = $(BINARY_DIR)/$(BINARY_NAME)

# Source files
SRC_DIR = .
MAIN_FILE = cmd/$(BINARY_NAME)/main.go
PKG_LIST = ./...

# Build flags
LDFLAGS = -ldflags="-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)"
BUILDFLAGS = $(LDFLAGS) -v

# Version parameters
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME = $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Default target
.PHONY: all
all: clean deps test build

# Clean up
.PHONY: clean
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BINARY_DIR)
	rm -rf vendor/
	rm -f coverage.out
	rm -f coverage.html

# Install dependencies
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Update dependencies
.PHONY: update-deps
update-deps:
	@echo "Updating dependencies..."
	$(GOMOD) tidy

# Build the application
.PHONY: build
build:
	@echo "Building $(BINARY_NAME) version $(VERSION) ($(GIT_COMMIT))..."
	mkdir -p $(BINARY_DIR)
	$(GOBUILD) $(BUILDFLAGS) -o $(BINARY_PATH) $(MAIN_FILE)
	@echo "Binary created at $(BINARY_PATH)"

# Run the application
.PHONY: run
run: build
	./$(BINARY_PATH) $(ARGS)

# Install the application
.PHONY: install
install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	@if [ "$(shell id -u)" -eq 0 ]; then \
		cp $(BINARY_PATH) /usr/local/bin/; \
	else \
		echo "You need root permissions to write to /usr/local/bin"; \
		echo "Running: sudo cp $(BINARY_PATH) /usr/local/bin/"; \
		sudo cp $(BINARY_PATH) /usr/local/bin/; \
	fi
	@echo "$(BINARY_NAME) successfully installed to /usr/local/bin/"

# User home directory installation (doesn't require sudo)
.PHONY: install-user
install-user: build
	@echo "Installing $(BINARY_NAME) to ~/bin..."
	@mkdir -p ~/bin
	@cp $(BINARY_PATH) ~/bin/
	@echo "$(BINARY_NAME) successfully installed to ~/bin/"
	@echo "Make sure ~/bin is in your PATH by adding this to your shell configuration:"
	@echo "export PATH=\$$PATH:~/bin"

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	$(GOTEST) -v $(PKG_LIST)

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out $(PKG_LIST)
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	$(GOCMD) tool cover -func=coverage.out
	@echo "Coverage report generated as coverage.html"

# Run tests with race detection (requires CGO)
.PHONY: test-race
test-race:
	@echo "Running tests with race detection..."
	CGO_ENABLED=1 $(GOTEST) -v -race $(PKG_LIST)

# Benchmarking
.PHONY: bench
bench:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem $(PKG_LIST)

# Run linters
.PHONY: lint
lint:
	@echo "Running linters..."
	$(GOVET) $(PKG_LIST)
	# Temporarily disable golint
	# $(GOLINT) $(PKG_LIST)

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	$(GOFMT) $(PKG_LIST)

# Security check
.PHONY: security-check
security-check:
	@echo "Running security checks..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec -quiet $(PKG_LIST); \
	else \
		echo "gosec not installed. Run: go install github.com/securego/gosec/v2/cmd/gosec@latest"; \
		exit 1; \
	fi

# Generate documentation
.PHONY: docs
docs:
	@echo "Generating API documentation..."
	@if command -v godoc >/dev/null 2>&1; then \
		godoc -http=:6060; \
	else \
		echo "godoc not installed. Run: go install golang.org/x/tools/cmd/godoc@latest"; \
		exit 1; \
	fi

# Install development tools
.PHONY: install-tools
install-tools:
	@echo "Installing development tools..."
	go install golang.org/x/lint/golint@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install golang.org/x/tools/cmd/godoc@latest

# Create a distribution package
.PHONY: dist
dist: build
	@echo "Creating distribution package..."
	mkdir -p dist
	tar -czvf dist/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz -C $(BINARY_DIR) $(BINARY_NAME)
	cp $(BINARY_PATH) dist/$(BINARY_NAME)-$(VERSION)
	@echo "Distribution package created in dist/"

# Check for outdated dependencies
.PHONY: outdated
outdated:
	@echo "Checking for outdated dependencies..."
	$(GOGET) -u github.com/psampaz/go-mod-outdated
	$(GOLIST) -u -m -json all | go-mod-outdated -update -direct

# Release a new version
.PHONY: release
release: clean test build dist
	@echo "Released version $(VERSION)"

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all              - Clean, install dependencies, test, and build"
	@echo "  clean            - Clean up build artifacts"
	@echo "  deps             - Install dependencies"
	@echo "  update-deps      - Update dependencies"
	@echo "  build            - Build the application"
	@echo "  run              - Run the application (builds first)"
	@echo "  install          - Install the application to /usr/local/bin (may require sudo)"
	@echo "  install-user     - Install the application to ~/bin (doesn't require sudo)"
	@echo "  test             - Run tests"
	@echo "  test-coverage    - Run tests with coverage and generate a report"
	@echo "  test-race        - Run tests with race detection (requires CGO)"
	@echo "  bench            - Run benchmarks"
	@echo "  lint             - Run linters"
	@echo "  fmt              - Format code"
	@echo "  security-check   - Run security checks"
	@echo "  docs             - Generate API documentation"
	@echo "  install-tools    - Install development tools"
	@echo "  dist             - Create a distribution package"
	@echo "  outdated         - Check for outdated dependencies"
	@echo "  release          - Release a new version"
	@echo "  help             - Show this help"
	@echo ""
	@echo "Examples:"
	@echo "  make build"
	@echo "  make run ARGS=\"monitor --once\""
	@echo "  make release VERSION=\"1.0.0\""