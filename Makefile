# Claude Code Go - Makefile
# Delegates to build.go for cross-platform builds

VERSION?=0.1.0-alpha

# ============================================================================
# Build targets (via build.go)
# ============================================================================

.PHONY: all
all: build

.PHONY: build
build:
	go run build.go -version $(VERSION)

.PHONY: build-all
build-all:
	go run build.go -action build-all -version $(VERSION)

.PHONY: release
release:
	go run build.go -action release -version $(VERSION)

.PHONY: clean
clean:
	go run build.go -action clean

.PHONY: test
test:
	go run build.go -action test -skip-tests=false

.PHONY: info
info:
	go run build.go -action info

# ============================================================================
# Direct Go commands (no build.go dependency)
# ============================================================================

.PHONY: deps
deps:
	go mod download
	go mod verify

.PHONY: install
install:
	go install -ldflags "-X main.Version=$(VERSION) -X claude-go/cmd.Version=$(VERSION) -X claude-go/internal/constants.Version=$(VERSION)" .

.PHONY: run
run: build
	./build/claude-go

.PHONY: test-coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

.PHONY: test-race
test-race:
	go test -v -race ./...

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: lint
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed"; \
	fi

.PHONY: check
check: fmt vet test
	@echo "All checks passed!"

.PHONY: help
help:
	@echo "Claude Code Go - Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build        Build for current platform"
	@echo "  build-all    Build for all platforms (6 targets)"
	@echo "  release      Create release archives with checksums"
	@echo "  clean        Clean build artifacts"
	@echo "  test         Run tests"
	@echo "  deps         Download dependencies"
	@echo "  install      Install to GOPATH/bin"
	@echo "  info         Show build environment info"
	@echo "  fmt          Format code"
	@echo "  vet          Run go vet"
	@echo "  check        Run fmt + vet + test"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION=$(VERSION)"
	@echo ""
	@echo "Examples:"
	@echo "  make build VERSION=1.0.0"
	@echo "  make release VERSION=1.0.0"
	@echo ""
	@echo "Or use build.go directly:"
	@echo "  go run build.go -action build-all -version 1.0.0"
	@echo "  go run build.go -action release -version 1.0.0"
	@echo "  go run build.go -os darwin -arch arm64"
