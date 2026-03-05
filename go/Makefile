.PHONY: all build test lint vet check clean install

INSTALL_DIR := $(HOME)/.local/bin

# Default: full pre-push check
all: check

# Build the binary
build:
	@echo "==> Building ai-review..."
	go build -o bin/ai-review ./cmd/ai-review
	@echo "    bin/ai-review built OK"

# Build and install to ~/.local/bin/
install: build
	@echo "==> Installing to $(INSTALL_DIR)/ai-review..."
	@mkdir -p $(INSTALL_DIR)
	cp bin/ai-review $(INSTALL_DIR)/ai-review
	chmod 755 $(INSTALL_DIR)/ai-review
	@echo "    Installed OK"

# Run all tests
test:
	@echo "==> Running tests..."
	go test ./... -count=1 -timeout 120s
	@echo "    All tests passed"

# Run go vet
vet:
	@echo "==> Running go vet..."
	go vet ./...
	@echo "    go vet OK"

# Run staticcheck if available
lint:
	@echo "==> Running lint..."
	@if command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./...; \
		echo "    staticcheck OK"; \
	else \
		echo "    staticcheck not installed, skipping (install: go install honnef.co/go/tools/cmd/staticcheck@latest)"; \
	fi

# Full pre-push check: vet + test + build
check: vet test build
	@echo ""
	@echo "==> All checks passed. Safe to push."

# Clean build artifacts
clean:
	rm -rf bin/
