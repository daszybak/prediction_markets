tgt_dir := target
src_dirs := cmd internal pkg

deps := $(shell find $(src_dirs) -name '*.go' -type f 2>/dev/null)
services := collector
# services := collector arbiter

.PHONY: all build test test-v bench check check_style check_lint fmt clean sqlc $(services)

all: build

# Build all services.
build: $(services)

# Build individual service (e.g., make collector).
$(services): %: $(tgt_dir)/%

$(tgt_dir)/%: $(deps) | $(tgt_dir)
	CGO_ENABLED=0 go build -o '$@' './cmd/$*'

# Run all tests.
test:
	go test ./...

# Run tests with verbose output.
test-v:
	go test -v ./...

# Run benchmarks.
bench:
	go test -bench=. -benchmem ./...

# Run all checks.
check: check_style check_lint test

# Check formatting.
check_style:
	@! (gofumpt -d $(src_dirs) 2>/dev/null | grep '')
	@! (goimports -d $(src_dirs) 2>/dev/null | grep '')

# Run linters.
check_lint:
	go vet ./...
	@command -v golangci-lint >/dev/null 2>&1 && \
		golangci-lint run ./... || \
		echo "golangci-lint not installed, skipping"

# Format all Go files.
fmt:
	gofumpt -w .
	goimports -w .

# Clean build artifacts.
clean:
	rm -rf $(tgt_dir)

# Generate sqlc code.
sqlc:
	sqlc generate

$(tgt_dir):
	mkdir -p '$@'
