default_service := "collector"

# List available recipes.
default:
    just --list

# Generate all configs from .env using config.sample.yaml templates.
config:
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ ! -f .env ]]; then
        echo "Error: .env file not found"
        exit 1
    fi
    set -a && source .env && set +a
    for sample in configs/*/config.sample.yaml; do
        dir=$(dirname "$sample")
        output="$dir/config.yaml"
        envsubst < "$sample" > "$output"
        echo "Generated $output"
    done

# ============================================================================
# Development (delegates to make for Go commands)
# ============================================================================

# Run a service locally.
run service=default_service *args:
    go run ./cmd/{{service}} -config=configs/{{service}}/config.yaml {{args}}

# Build Go binaries.
build *args:
    make build {{args}}

# Run tests.
test *args:
    make test {{args}}

# Run linters.
check *args:
    make check {{args}}

# Format code.
fmt:
    make fmt

# Generate Go code from SQL queries.
sqlc:
    sqlc generate

# Clean build artifacts.
clean:
    make clean
    rm -rf tmp

# ============================================================================
# Docker Compose
# ============================================================================

# Start dev environment with hot reload (rebuilds if Dockerfile changed)
up *args:
    docker compose up --build {{args}}

# Stop all services.
down *args:
    docker compose down {{args}}

# View logs (use: just logs, just logs collector, just logs -f).
logs *args:
    docker compose logs {{args}}

# Start only dependencies (db, redis) for local Go development.
deps *args:
    docker compose up -d timescaledb redis {{args}}

# ============================================================================
# Container execution
# ============================================================================

# Execute a command in a running service container.
exec service=default_service *args:
    docker compose exec {{service}} {{args}}

# Open shell in service container.
shell service=default_service:
    docker compose exec {{service}} sh

# ============================================================================
# Database
# ============================================================================

# Connect to TimescaleDB.
db:
    docker compose exec timescaledb psql -U $POSTGRES_USER -d $POSTGRES_DB

# Connect to Redis CLI.
redis:
    docker compose exec redis redis-cli

# Run migrations via container (e.g., just migrate, just migrate down 1, just migrate version).
migrate *args='up':
    docker compose run --rm migrate {{args}}

# Create new migration files locally.
migrate-create name:
    migrate create -ext sql -dir db/migrations -seq {{name}}

# ============================================================================
# Production
# ============================================================================

# Build production image for a service.
image service=default_service:
    docker build --build-arg SERVICE={{service}} --target runtime -t prediction_markets-{{service}}:latest .

# Run production stack (no override file).
prod *args:
    docker compose -f compose.yaml up {{args}}

# ============================================================================
# CI (used by scripts/with_build_env.sh)
# ============================================================================

# Build the CI image.
ci-image:
    docker build --target builder -t prediction_markets-build .

# Run command in CI container.
ci *args:
    bash scripts/with_build_env.sh {{args}}
