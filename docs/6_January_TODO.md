# Project Status - January 2026

## Completed

### Engine (`internal/engine/`)

- [x] In-memory order book per token
- [x] Goroutine-per-token architecture (no mutex contention across tokens)
- [x] Channel-based update routing
- [x] `Set` for absolute updates (Polymarket snapshots)
- [x] `Update` for delta updates (Kalshi increments)
- [x] `price.Price` and `price.Size` types (int64 with 10^6 scale)
- [x] `google/btree` for ordered orderbooks (O(log n) insert/retrieval)
- [x] `GetTopN` method for retrieving top N price levels
- [x] `TakeSnapshots` method for snapshot writer integration
- [x] `Send` method for platforms to push updates
- [x] RWMutex for thread-safe worker map access

### Snapshot Writer (`internal/engine/snapshot.go`)

- [x] Periodic snapshot ticker (configurable interval)
- [x] Batch insert via `InsertOrderBookSnapshotBatch` (COPY protocol)
- [x] Configurable snapshot depth (top N levels per side)
- [x] Event time tracking per level (`UpdatedAt` from source API)

### Polymarket Platform (`internal/polymarket/`)

- [x] WebSocket client with configurable URL + endpoint separation
- [x] Full message parsing for all event types:
  - `book` - Order book snapshots
  - `price_change` - Price level updates
  - `tick_size_change` - Tick size changes
  - `best_bid_ask` - Top of book updates
  - `new_market` - New market announcements
  - `market_resolved` - Market resolution events
- [x] CLOB client for fetching all markets
- [x] Market sync loop with configurable interval
- [x] Token subscription to websocket
- [x] Platform Start/Stop lifecycle
- [x] Structured logging with slog

### Database & Store (`internal/store/`)

- [x] TimescaleDB connection pool (pgx)
- [x] sqlc code generation
- [x] Migrations (000001-000006):
  - markets, tokens tables
  - order_book_snapshots hypertable
  - order_book_metrics hypertable
  - trades hypertable
  - market_pairs table (for cross-platform matching)
  - `ingested_at` column for latency tracking
- [x] pgvector extension enabled
- [x] Market embeddings table schema

### Configuration (`cmd/collector/config.go`)

- [x] YAML config with environment variable substitution
- [x] Config validation
- [x] Duration parsing for intervals
- [x] WebSocket config with URL + market endpoint
- [x] Engine config (snapshot_interval, snapshot_depth)

### Packages

- [x] `internal/price` - int64 Price and Size types with 10^6 scale
- [x] `pkg/hashset` - Generic hashset implementation

### Infrastructure

- [x] Docker Compose with TimescaleDB
- [x] Migrations via golang-migrate
- [x] Hot reload with Air
- [x] Justfile commands

---

## In Progress

### Polymarket WebSocket → Engine

- [ ] Route parsed messages to Engine
- [ ] Reconnection logic with exponential backoff

### TimescaleDB Continuous Aggregates

- [ ] Create continuous aggregates for OHLC rollups (1m, 5m, 1h, 1d)

---

## Next Up

### Kalshi Platform

- [x] API client structure (`internal/kalshi/api/client.go`)
- [ ] WebSocket client
- [ ] Market sync from REST API
- [ ] Authenticate with RSA private key
- [ ] Delta updates to Engine

### LLM Service

Architecture documented in `docs/llm-service-architecture.md`.

- [ ] Implement `internal/llm/` package
- [ ] Provider implementations (Anthropic, OpenAI, Ollama)
- [ ] Market rule parsing prompts
- [ ] Market equivalence verification
- [ ] Budget tracking

### Market Matching Pipeline

- [ ] Embedding generation (sentence-transformers via Python sidecar or Go)
- [ ] pgvector similarity search
- [ ] LLM verification for top candidates
- [ ] Cache results in `market_pairs` table

---

## Architecture Notes

### Engine Flow (Hot Path)

```
WebSocket ──▶ Collector ──▶ Engine.Send(Update)
                                    │
                                    ▼
                              Engine.Start()
                                    │
                    ┌───────────────┼───────────────┐
                    │               │               │
                    ▼               ▼               ▼
             TokenA worker   TokenB worker   TokenC worker
                    │               │               │
                    ▼               ▼               ▼
             TokenA btree    TokenB btree    TokenC btree
             (bids/asks)     (bids/asks)     (bids/asks)
                    │               │               │
                    └───────────────┼───────────────┘
                                    │
                          SnapshotWriter.Start()
                          (ticker @ configured interval)
                                    │
                            TakeSnapshots(depth)
                                    │
                                    ▼
                      InsertOrderBookSnapshotBatch
                                    │
                                    ▼
                              TimescaleDB
```

### Why No Redis?

- **Latency**: In-memory maps are nanoseconds, Redis is milliseconds
- **Complexity**: No external dependency to manage
- **Bottleneck**: WebSocket feed is 10-100ms jittery anyway
- **Recovery**: Order book rebuilt from WebSocket on reconnect

Redis would only help if:
- Multiple services need real-time order book (not yet)
- Horizontal scaling required (not yet)

### TimescaleDB Snapshot Strategy

Two timestamps per row for latency analysis:
- `time` = event time (when data was generated at source API)
- `ingested_at` = ingestion time (when we stored it, defaults to NOW())
- Latency = `ingested_at - time`

```sql
-- Periodic snapshots with event time from source
INSERT INTO order_book_snapshots (time, token_id, side, level, price, size)
VALUES ($1, $2, $3, $4, $5, $6);
-- ingested_at column uses DEFAULT NOW()

-- Query latency distribution
SELECT
    token_id,
    avg(ingested_at - time) as avg_latency,
    percentile_cont(0.99) WITHIN GROUP (ORDER BY ingested_at - time) as p99_latency
FROM order_book_snapshots
WHERE time > NOW() - INTERVAL '1 hour'
GROUP BY token_id;

-- Continuous aggregate refreshes automatically
CREATE MATERIALIZED VIEW order_book_1m
WITH (timescaledb.continuous) AS
SELECT time_bucket('1 minute', time), token_id, ...
FROM order_book_snapshots
GROUP BY 1, 2;
```

### Price Representation

All prices stored as `int64` with 10^6 scale (micro-dollars):
- `$0.55` → `550000`
- `$0.01` → `10000`
- `$0.999999` → `999999`

See `internal/price/price.go`.

---

## Key Files

| File | Purpose |
|------|---------|
| `cmd/collector/main.go` | Service entrypoint |
| `cmd/collector/config.go` | Config loading and validation |
| `internal/engine/client.go` | Engine router, goroutine-per-token, TakeSnapshots |
| `internal/engine/snapshot.go` | SnapshotWriter with periodic batch inserts |
| `internal/engine/orderbook/orderbook.go` | Btree-backed orderbook (bids desc, asks asc) |
| `internal/polymarket/polymarket.go` | Platform implementation |
| `internal/polymarket/websocket/client.go` | WebSocket client + message types |
| `internal/polymarket/clob/client.go` | REST API client |
| `internal/store/store.go` | Database queries |
| `internal/price/price.go` | Price and Size types |
