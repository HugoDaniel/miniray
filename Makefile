# miniray Makefile

# Build settings
BINARY_NAME := miniray
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go settings
GOFLAGS := -trimpath
LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT)

# Directories
BUILD_DIR := build
CMD_DIR := cmd/miniray

.PHONY: all build clean test lint fmt vet check install help build-wasm package-wasm

# Default target
all: check build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)"

# Build for multiple platforms
build-all: build-linux build-darwin build-windows build-wasm

build-linux:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./$(CMD_DIR)
	GOOS=linux GOARCH=arm64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./$(CMD_DIR)

build-darwin:
	@echo "Building for macOS..."
	GOOS=darwin GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./$(CMD_DIR)
	GOOS=darwin GOARCH=arm64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./$(CMD_DIR)

build-windows:
	@echo "Building for Windows..."
	GOOS=windows GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./$(CMD_DIR)

# Build WebAssembly for browser/Node.js
NPM_DIR := npm/miniray
WASM_CMD := cmd/miniray-wasm

build-wasm:
	@echo "Building WebAssembly..."
	@mkdir -p $(BUILD_DIR)
	GOOS=js GOARCH=wasm go build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME).wasm ./$(WASM_CMD)
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME).wasm ($$(du -h $(BUILD_DIR)/$(BINARY_NAME).wasm | cut -f1))"

# Package WASM for npm distribution
package-wasm: build-wasm
	@echo "Packaging WASM for npm..."
	@mkdir -p $(NPM_DIR)
	@cp $(BUILD_DIR)/$(BINARY_NAME).wasm $(NPM_DIR)/
	@cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" $(NPM_DIR)/
	@cp "$$(go env GOROOT)/lib/wasm/wasm_exec_node.js" $(NPM_DIR)/ 2>/dev/null || true
	@echo "NPM package ready in $(NPM_DIR)/"
	@echo "Files:"
	@ls -lh $(NPM_DIR)/*.wasm $(NPM_DIR)/*.js 2>/dev/null || true

# Install to GOPATH/bin
install:
	go install $(GOFLAGS) -ldflags "$(LDFLAGS)" ./$(CMD_DIR)

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	go clean -testcache

# Run tests
test:
	@echo "Running tests..."
	go test -v -race ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@mkdir -p $(BUILD_DIR)
	go test -v -race -coverprofile=$(BUILD_DIR)/coverage.out ./...
	go tool cover -html=$(BUILD_DIR)/coverage.out -o $(BUILD_DIR)/coverage.html
	@echo "Coverage report: $(BUILD_DIR)/coverage.html"

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Run linter
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	fi

# Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

# Run all checks
check: fmt vet
	@echo "All checks passed"

# Generate code (if needed)
generate:
	go generate ./...

# Run the minifier on a test file
run: build
	@if [ -f testdata/example.wgsl ]; then \
		./$(BUILD_DIR)/$(BINARY_NAME) testdata/example.wgsl; \
	else \
		echo "No test file found. Create testdata/example.wgsl"; \
	fi

# Show help
help:
	@echo "miniray Build System"
	@echo ""
	@echo "Targets:"
	@echo "  build         Build the binary"
	@echo "  build-all     Build for all platforms (including WASM)"
	@echo "  build-wasm    Build WebAssembly module"
	@echo "  package-wasm  Package WASM for npm distribution"
	@echo "  install       Install to GOPATH/bin"
	@echo "  clean         Remove build artifacts"
	@echo "  test          Run tests"
	@echo "  test-coverage Run tests with coverage"
	@echo "  bench         Run benchmarks"
	@echo "  lint          Run linter"
	@echo "  fmt           Format code"
	@echo "  vet           Run go vet"
	@echo "  check         Run all checks"
	@echo "  help          Show this help"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION=$(VERSION)"
	@echo "  COMMIT=$(COMMIT)"
