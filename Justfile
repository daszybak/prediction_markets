tgt_dir := "target"
src_dirs := "cmd internal pkg"

# List available recipes.
default:
    just --list

# Build the main binary.
build:
    make build

# Run all tests.
test pattern=".*":
    go test -run '{{pattern}}' ./cmd/... ./internal/... ./pkg/...

# Run tests with verbose output.
test-v pattern=".*":
    go test -v -run '{{pattern}}' ./cmd/... ./internal/... ./pkg/...

# Run benchmarks.
bench pattern=".*":
    go test -bench='{{pattern}}' -benchmem ./cmd/... ./internal/... ./pkg/...

# Run all checks (style, lint, tests).
check: check_style check_lint test

# Check Go formatting.
check_style:
    @! (gofumpt -d {{src_dirs}} 2>/dev/null | grep '')
    @! (goimports -d {{src_dirs}} 2>/dev/null | grep '')
    @! (gofmt -s -d {{src_dirs}} 2>/dev/null | grep '')

# Run linters.
check_lint:
    go vet ./...
    golangci-lint run ./...

# Format all Go files.
fmt:
    gofumpt -w .
    goimports -w .

# Clean build artifacts.
clean:
    rm -rf {{tgt_dir}}

# Run the collector.
run *args:
    go run ./cmd/collector {{args}}

# Build the Docker build environment image.
docker-build:
    docker build --target builder -t prediction_markets-build .

# Run a command in the Docker build environment.
docker-run *args:
    bash scripts/with_build_env.sh {{args}}

# Run tests in Docker.
docker-test:
    bash scripts/with_build_env.sh make test

# Run checks in Docker (for CI).
docker-check:
    bash scripts/with_build_env.sh make check

# Open a shell in the Docker build environment.
docker-shell:
    bash scripts/with_build_env.sh bash

# Build the production image.
docker-build-prod:
    docker build --target runtime -t prediction_markets:latest .
