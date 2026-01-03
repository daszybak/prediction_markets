# Prediction Markets

High-frequency data collection and analysis for prediction market platforms.

## Prerequisites

- Docker & Docker Compose
- Go 1.25+ (for local development)
- [just](https://github.com/casey/just) (command runner)
- [golang-migrate](https://github.com/golang-migrate/migrate) (for creating new migrations locally)
- [sqlc](https://sqlc.dev/) (for generating Go code from SQL)

## Quick Start

### 1. Setup Environment

```bash
cp .env.sample .env
# Edit .env with your values (API keys, etc.)
```

### 2. Development

```bash
# Start all services with hot reload
just up

# Or start only dependencies (DB, Redis) for local Go development
just deps
just run  # Runs collector locally
```

**What happens:**
- TimescaleDB and Redis start with health checks
- Config auto-generated from `.env` via `envsubst`
- Air watches for file changes and rebuilds

### 3. Production

```bash
# Run production stack (no hot reload, includes migrations)
just prod
```

**What happens:**
- `migrate` container runs first, applies all migrations
- Services start after migrations complete
- Configs generated from environment variables

## Project Structure

```
├── cmd/
│   └── collector/          # Service entrypoints
├── configs/
│   └── collector/
│       ├── config.sample.yaml  # Template (committed)
│       ├── config.yaml         # Local config (gitignored)
│       └── air.toml            # Hot reload config
├── db/
│   └── migrations/         # SQL migrations (golang-migrate format)
├── internal/               # Private packages
├── pkg/                    # Shared packages
└── scripts/
    ├── entrypoint.sh       # Production entrypoint
    ├── entrypoint.dev.sh   # Development entrypoint
    └── migrate.sh          # Migration runner
```

## Commands

### Development

| Command | Description |
|---------|-------------|
| `just up` | Start dev environment (hot reload) |
| `just down` | Stop all services |
| `just deps` | Start only DB and Redis |
| `just run` | Run collector locally |
| `just logs` | View logs |
| `just db` | Connect to TimescaleDB |
| `just redis` | Connect to Redis CLI |

### Database

| Command | Description |
|---------|-------------|
| `just migrate` | Run all pending migrations (via container) |
| `just migrate down 1` | Rollback last migration |
| `just migrate down -all` | Rollback all migrations |
| `just migrate version` | Show current migration version |
| `just migrate-create name` | Create new migration files locally |

### Build & Test

| Command | Description |
|---------|-------------|
| `just build` | Build binaries |
| `just test` | Run tests |
| `just check` | Run linters |
| `just fmt` | Format code |
| `just sqlc` | Generate Go code from SQL queries |

### Production

| Command | Description |
|---------|-------------|
| `just prod` | Run production stack |
| `just image` | Build production image |
| `just image arbiter` | Build specific service image |

## Adding a New Service

1. Create service code:
   ```bash
   mkdir -p cmd/arbiter
   # Add main.go
   ```

2. Create service config:
   ```bash
   mkdir -p configs/arbiter
   cp configs/collector/config.sample.yaml configs/arbiter/
   cp configs/collector/air.toml configs/arbiter/
   # Edit air.toml: change collector -> arbiter
   ```

3. Add to compose.yaml:
   ```yaml
   arbiter:
     build:
       context: .
       target: runtime
       args:
         SERVICE: arbiter
     env_file: .env
     environment:
       - SERVICE=arbiter
     depends_on:
       migrate:
         condition: service_completed_successfully
   ```

4. Add to compose.override.yaml for dev.

## Environment Variables

See `.env.sample` for all available variables.

**Required:**
- `POSTGRES_PASSWORD` - Database password

**Platform configs:**
- `POLYMARKET_*` - Polymarket API settings
- `KALSHI_*` - Kalshi API settings

## Architecture

```
┌────────────────────────────────────────────────────────────────────┐
│                         DATA SOURCES                                │
├────────────┬────────────┬────────────┬────────────┬───────────────┤
│ Polymarket │   Kalshi   │ PredictIt  │  Metaculus │  News Feeds   │
│ (WebSocket)│   (API)    │   (API)    │   (API)    │  (RSS/APIs)   │
└─────┬──────┴─────┬──────┴─────┬──────┴─────┬──────┴───────┬───────┘
      │            │            │            │              │
      └────────────┴────────────┼────────────┴──────────────┘
                                │
                     ┌──────────▼──────────┐
                     │      Collector      │
                     │  (Go, real-time)    │
                     └──────────┬──────────┘
                                │
          ┌─────────────────────┼─────────────────────┐
          │                     │                     │
   ┌──────▼──────┐       ┌──────▼──────┐      ┌──────▼──────┐
   │    Redis    │       │ TimescaleDB │      │  pgvector   │
   │ (real-time) │       │(time-series)│      │ (semantic)  │
   │  prices,    │       │  historical │      │  market     │
   │  orderbook  │       │  analysis   │      │  matching   │
   └──────┬──────┘       └──────┬──────┘      └──────┬──────┘
          │                     │                    │
          └─────────────────────┼────────────────────┘
                                │
                     ┌──────────▼──────────┐
                     │      Arbiter        │
                     │  - Signal detection │
                     │  - Arbitrage finder │
                     │  - Execution        │
                     └─────────────────────┘
```

## Data Flow

| Path | Data | Latency | Storage |
|------|------|---------|---------|
| **Hot** | Prices, spreads, orderbook | <1ms | Redis |
| **Warm** | Time-series, metrics | <100ms | TimescaleDB |
| **Cold** | Market matching, news correlation | <500ms | pgvector |

## Supported Platforms

| Platform | Type | Status |
|----------|------|--------|
| Polymarket | DeFi/Crypto | Active |
| Kalshi | US Regulated | Active |
| PredictIt | US Political | Planned |
| Metaculus | Community | Planned |
| Manifold | Play Money | Planned |

## Key Features

- **Cross-platform arbitrage**: Match equivalent markets via pgvector embeddings + LLM verification
- **News signals**: Correlate news → markets using semantic search
- **Real-time collection**: WebSocket streams for orderbook depth
- **Time-series analysis**: TimescaleDB hypertables with compression
