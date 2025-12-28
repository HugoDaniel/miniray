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

.PHONY: all build clean test lint fmt vet check install help build-wasm package-wasm lib lib-clean \
        coverage coverage-html coverage-report coverage-func coverage-check coverage-clean

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

# Build C static library for FFI integration
LIB_CMD := cmd/miniray-lib

.PHONY: lib lib-clean

lib:
	@echo "Building C static library (libminiray.a)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 go build -buildmode=c-archive \
		-o $(BUILD_DIR)/libminiray.a ./$(LIB_CMD)
	@echo "Built:"
	@echo "  $(BUILD_DIR)/libminiray.a  ($$(du -h $(BUILD_DIR)/libminiray.a | cut -f1))"
	@echo "  $(BUILD_DIR)/libminiray.h  (C header)"

lib-clean:
	@rm -f $(BUILD_DIR)/libminiray.a $(BUILD_DIR)/libminiray.h

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

# Coverage settings
COVERAGE_DIR := $(BUILD_DIR)/coverage
COVERAGE_PROFILE := $(COVERAGE_DIR)/coverage.out
COVERAGE_HTML := $(COVERAGE_DIR)/coverage.html
COVERAGE_THRESHOLD := 50

# Run tests with coverage (quick summary)
coverage:
	@echo "Running tests with coverage..."
	@mkdir -p $(COVERAGE_DIR)
	@go test -coverprofile=$(COVERAGE_PROFILE) -covermode=atomic ./... 2>&1 | \
		grep -E '^(ok|FAIL|\?)' | \
		awk '{if ($$1 == "ok" || $$1 == "FAIL") { split($$0, a, "coverage:"); if (length(a) > 1) print $$2, a[2]; else print $$2, "no test files" } }'
	@echo ""
	@echo "Total coverage: $$(go tool cover -func=$(COVERAGE_PROFILE) | grep total | awk '{print $$3}')"

# Generate HTML coverage report
coverage-html: coverage
	@go tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML)
	@echo "HTML report: $(COVERAGE_HTML)"
	@if command -v open >/dev/null 2>&1; then open $(COVERAGE_HTML); fi

# Show per-function coverage
coverage-func:
	@if [ ! -f $(COVERAGE_PROFILE) ]; then $(MAKE) coverage; fi
	@go tool cover -func=$(COVERAGE_PROFILE)

# Detailed coverage report by package
coverage-report:
	@echo "Running coverage analysis..."
	@mkdir -p $(COVERAGE_DIR)
	@go test -count=1 -coverprofile=$(COVERAGE_PROFILE) -covermode=atomic ./... > /dev/null 2>&1
	@echo ""
	@echo "═══════════════════════════════════════════════════════════════"
	@echo "                    COVERAGE REPORT"
	@echo "═══════════════════════════════════════════════════════════════"
	@echo ""
	@go test -count=1 -cover ./... 2>&1 | \
		awk -F'\t' '/^ok|^\?/ { \
			pkg = $$2; \
			gsub(/github.com\/HugoDaniel\/miniray\//, "", pkg); \
			if (pkg == "") pkg = "."; \
			if (/no test files/) { cov = "no tests" } \
			else if (/coverage:/) { \
				match($$0, /coverage: [0-9.]+%/); \
				cov = substr($$0, RSTART+10, RLENGTH-10); \
			} else { cov = "0.0%" } \
			printf "  %-40s %s\n", pkg, cov \
		}'
	@echo ""
	@echo "───────────────────────────────────────────────────────────────"
	@total=$$(go tool cover -func=$(COVERAGE_PROFILE) | grep total | awk '{print $$3}'); \
	echo "  TOTAL                                    $$total"
	@echo "═══════════════════════════════════════════════════════════════"
	@go tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML)
	@echo ""
	@echo "HTML report: $(COVERAGE_HTML)"

# Check coverage meets threshold
coverage-check:
	@echo "Checking coverage threshold ($(COVERAGE_THRESHOLD)%)..."
	@mkdir -p $(COVERAGE_DIR)
	@go test -coverprofile=$(COVERAGE_PROFILE) -covermode=atomic ./... > /dev/null 2>&1
	@total=$$(go tool cover -func=$(COVERAGE_PROFILE) | grep total | awk '{gsub(/%/, ""); print $$3}'); \
	echo "Current coverage: $$total%"; \
	if [ $$(echo "$$total < $(COVERAGE_THRESHOLD)" | bc -l) -eq 1 ]; then \
		echo "FAIL: Coverage $$total% is below threshold $(COVERAGE_THRESHOLD)%"; \
		exit 1; \
	else \
		echo "PASS: Coverage meets threshold"; \
	fi

# Clean coverage files
coverage-clean:
	@rm -rf $(COVERAGE_DIR)
	@echo "Coverage files cleaned"

# Legacy alias
test-coverage: coverage-html

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
	@echo "  lib           Build C static library (libminiray.a) for FFI"
	@echo "  install       Install to GOPATH/bin"
	@echo "  clean         Remove build artifacts"
	@echo "  test          Run tests"
	@echo "  coverage      Quick coverage summary"
	@echo "  coverage-html Generate HTML coverage report"
	@echo "  coverage-report Detailed coverage by package"
	@echo "  coverage-func Per-function coverage breakdown"
	@echo "  coverage-check Check coverage meets threshold ($(COVERAGE_THRESHOLD)%)"
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
