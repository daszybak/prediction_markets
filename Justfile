default_service := "collector"

# List available recipes.
default:
    just --list

# ============================================================================
# Development (delegates to make for Go commands)
# ============================================================================

# Run a service locally.
run service=default_service *args:
    go run ./cmd/{{service}} -config=configs/{{service}}/config.yaml {{args}}

# Delegate to make for build/test/lint.
build *args:
    make build {{args}}

test *args:
    make test {{args}}

check *args:
    make check {{args}}

fmt:
    make fmt

clean:
    make clean
    rm -rf tmp

# ============================================================================
# Docker Compose
# ============================================================================

# Start all services (dev mode with hot reload).
up *args:
    docker compose up {{args}}

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

# Database URL for migrations (local dev)
db_url := "postgres://${POSTGRES_USER:-prediction}:${POSTGRES_PASSWORD}@localhost:5432/${POSTGRES_DB:-prediction}?sslmode=disable"

# Run all pending migrations.
migrate:
    migrate -path db/migrations -database "{{db_url}}" up

# Rollback last migration.
migrate-down:
    migrate -path db/migrations -database "{{db_url}}" down 1

# Rollback all migrations.
migrate-reset:
    migrate -path db/migrations -database "{{db_url}}" down -all

# Show migration version.
migrate-version:
    migrate -path db/migrations -database "{{db_url}}" version

# Force set migration version (use with caution).
migrate-force version:
    migrate -path db/migrations -database "{{db_url}}" force {{version}}

# Create new migration files.
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
