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

# Or start only dependencies (DB) for local Go development
just deps
just run  # Runs collector locally
```

**What happens:**
- TimescaleDB starts with health checks
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
| `just deps` | Start only DB |
| `just run` | Run collector locally |
| `just logs` | View logs |
| `just db` | Connect to TimescaleDB |

### Database

| Command | Description |
|---------|-------------|
| `just migrate` | Run all pending migrations (via container) |
| `just migrate down 1` | Rollback last migration |
| `just migrate down -all` | Rollback all migrations |
| `just migrate drop -f` | Drop everything (tables, data) |
| `just migrate version` | Show current migration version |
| `just migrate force N` | Force set version (fix dirty state) |
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
- `POLYMARKET_WS_URL` - WebSocket endpoint
- `POLYMARKET_GAMMA_URL` - Gamma API (market metadata)
- `POLYMARKET_CLOB_URL` - CLOB API (orderbook)
- `POLYMARKET_MARKET_SYNC_INTERVAL` - How often to sync markets (e.g., `5m`)
- `KALSHI_*` - Kalshi API settings

**Engine configs:**
- `ENGINE_SNAPSHOT_INTERVAL` - How often to snapshot orderbooks to DB (e.g., `10s`)
- `ENGINE_SNAPSHOT_DEPTH` - Number of price levels per side to capture (e.g., `10`)

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
   │   Engine    │       │ TimescaleDB │      │  pgvector   │
   │ (in-memory) │       │(time-series)│      │ (semantic)  │
   │  orderbooks │       │  snapshots, │      │  market     │
   │  per token  │       │  analysis   │      │  matching   │
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

| Path | Data | Latency | Storage | Status |
|------|------|---------|---------|--------|
| **Hot** | Prices, spreads, orderbook | <1µs | In-memory Engine | ✅ Active |
| **Warm** | Time-series snapshots | <100ms | TimescaleDB | ✅ Active |
| **Cold** | Market matching, news correlation | <500ms | pgvector | 🚧 Schema ready |

## Supported Platforms

| Platform | Type | Status |
|----------|------|--------|
| Polymarket | DeFi/Crypto | ✅ Active (WebSocket orderbook with full message parsing, REST market sync, token subscription) |
| Kalshi | US Regulated | 🚧 In Progress (API client started, WebSocket pending) |
| PredictIt | US Political | Planned |
| Metaculus | Community | Planned |
| Manifold | Play Money | Planned |

## Related Projects

- [Polymarket/agents](https://github.com/Polymarket/agents) - Official AI agents with Chroma vectorization
- [awesome-prediction-markets](https://github.com/0xperp/awesome-prediction-markets) - Curated list of tools
- [PredictOS](https://github.com/PredictionXBT/PredictOS) - Opensource prediction market framework

## Key Features

- **In-memory order books**: Goroutine-per-token architecture, no external cache needed
- **Cross-platform arbitrage**: Match equivalent markets via pgvector embeddings + LLM verification
- **News signals**: Correlate news → markets using semantic search
- **Real-time collection**: WebSocket streams for orderbook depth
- **Time-series snapshots**: Periodic order book snapshots to TimescaleDB with continuous aggregates

## Engine Architecture

The hot path uses in-memory order books with a dedicated goroutine per token:

```
WebSocket ──▶ Collector ──▶ Engine Router ─┬──▶ chan ──▶ BTC worker ──▶ BTC orderbook (btree)
                                           ├──▶ chan ──▶ ETH worker ──▶ ETH orderbook (btree)
                                           └──▶ chan ──▶ SOL worker ──▶ SOL orderbook (btree)
                                                              │
                                                      SnapshotWriter (ticker)
                                                              │
                                                              ▼
                                                    InsertOrderBookSnapshotBatch
                                                              │
                                                              ▼
                                                       TimescaleDB
```

- **No Redis**: Order book state lives in-memory (nanosecond access)
- **No mutex contention**: Each token has its own goroutine and channel
- **Parallel processing**: Updates to different tokens never block each other
- **Btree orderbooks**: `google/btree` for O(log n) insert and sorted retrieval
- **Periodic persistence**: SnapshotWriter runs on configurable interval, batch inserts to TimescaleDB

## TimescaleDB Snapshots

Order book snapshots use TimescaleDB hypertables with continuous aggregates:

```sql
-- Raw snapshots (every N seconds)
CREATE TABLE order_book_snapshots (
    event_time  TIMESTAMPTZ NOT NULL,  -- When event occurred at source
    token_id    TEXT NOT NULL,
    side        TEXT NOT NULL,
    price       BIGINT NOT NULL,
    size        BIGINT NOT NULL,
    ingested_at TIMESTAMPTZ            -- When we received from WebSocket
);
SELECT create_hypertable('order_book_snapshots', 'event_time');

-- Continuous aggregate for OHLC (auto-refreshed)
CREATE MATERIALIZED VIEW order_book_1m
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 minute', event_time) AS bucket,
    token_id,
    first(price, event_time) as open,
    max(price) as high,
    min(price) as low,
    last(price, event_time) as close,
    sum(size) as volume
FROM order_book_snapshots
WHERE side = 'bids'
GROUP BY bucket, token_id;
```

Best practices:
- Chunk interval: ~1 day for order book data
- Continuous aggregates: 1m, 5m, 1h, 1d rollups
- Compression: Enable after 7 days
