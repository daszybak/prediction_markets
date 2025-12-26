tgt_dir := target
pkg := github.com/daszybak/prediction_markets
src_dirs := cmd internal pkg

deps := $(shell find $(src_dirs) -name '*.go' -type f 2>/dev/null)

.PHONY: all build test test-v bench check check_style check_lint fmt clean

all: build

# Build the collector binary.
build: $(tgt_dir)/collector

$(tgt_dir)/collector: $(deps) | $(tgt_dir)
	( \
		cd cmd/collector && \
		CGO_ENABLED=0 go build -o '../../$@' \
	)

# Run all tests.
test:
	go test ./cmd/... ./internal/... ./pkg/...

# Run tests with verbose output.
test-v:
	go test -v ./cmd/... ./internal/... ./pkg/...

# Run benchmarks.
bench:
	go test -bench=. -benchmem ./cmd/... ./internal/... ./pkg/...

# Run all checks.
check: check_style check_lint test

# Check formatting.
check_style:
	@! (gofumpt -d $(src_dirs) 2>/dev/null | grep '')
	@! (goimports -d $(src_dirs) 2>/dev/null | grep '')
	@! (gofmt -s -d $(src_dirs) 2>/dev/null | grep '')

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

$(tgt_dir):
	mkdir -p '$@'
