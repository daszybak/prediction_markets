# TODO - January 6, 2025

## Completed Today

- [x] Config validation in `cmd/collector/config.go`
- [x] Polymarket Platform implementation with Start/Stop lifecycle
- [x] `syncMarkets()` - fetches markets from CLOB API and upserts to database
- [x] `GetTokenIDsForPlatform` SQL query for fetching token IDs by platform
- [x] Fixed `store.go` - `newQueries` -> `New` (sqlc naming)
- [x] Fixed Dockerfile migrate stage - `ENTRYPOINT` + `CMD` for passing args
- [x] Added `UNIQUE(market_id, outcome)` constraint to tokens table
- [x] Updated Justfile with migrate command examples

## Previously Completed

- [x] Fix race condition in WebSocket ReadMessage (using result channel)
- [x] Basic graceful shutdown with signal handling in main.go
- [x] Non-fatal error handling in main loop (log.Printf instead of log.Fatalf)
- [x] Goroutine cleanup on context cancel (SetReadDeadline trick)
- [x] Handle base64 decode error in pagination (clob/client.go:90-93)

---

## In Progress

### Polymarket Platform (internal/polymarket/polymarket.go)

Current implementation:
- `New()` - creates client with config and store
- `Start()` - syncs markets, connects websocket, subscribes, reads messages (blocking)
- `Stop()` - closes websocket
- `syncMarkets()` - upserts markets and tokens to database

**TODOs in code:**
- [ ] Fetch `end_date` from gamma API (clob doesn't have it)
- [x] Start goroutine to periodically sync markets (`syncLoop`)
- [ ] Process incoming messages (update order book, record trades)
- [ ] Reconnection logic with backoff

---

## High Priority

- [ ] Message processing - parse websocket messages and store order book/trades
- [x] Periodic market sync goroutine (`syncLoop` with configurable interval)
- [ ] Implement Kalshi client (uses same ticker pattern as Polymarket)
- [x] Add structured logging (log/slog) - implemented with component tagging

---

## Medium Priority

- [ ] Add health check HTTP endpoint
- [ ] Add Prometheus metrics
- [ ] Cache market list to avoid re-fetch on restart
- [ ] Circuit breaker pattern for external API calls
- [ ] Reconnection with exponential backoff

---

## Low Priority

- [ ] Request rate limiting
- [ ] Request correlation IDs
- [ ] Trading execution service (arbiter)
- [x] Platform interface abstraction (`internal/platform/platform.go` with Start/Stop methods)

---

## Architecture Notes

### Current Flow

```
main.go
  └── collector.platforms["polymarket"] = polymarket.New(cfg, store, logger)
  └── platform.Start(ctx)
        ├── websocket.New()           # Connect WS
        ├── go syncLoop(ctx)          # Background goroutine:
        │     ├── syncMarkets()       #   - Initial sync
        │     ├── subscribeToMarkets()#   - Initial subscribe
        │     └── ticker.C → sync     #   - Periodic re-sync (configurable interval)
        └── for { ReadMessage }       # Blocking read loop
```

### Token ID Strategy

For multi-platform support:
- **Polymarket**: Native `token_id` from API
- **Kalshi**: Synthetic `{ticker}_YES` / `{ticker}_NO`
- **PredictIt**: Native contract `id`

Database schema uses `UNIQUE(market_id, outcome)` to enforce one token per outcome per market.

### Key Files Modified

| File | Change |
|------|--------|
| `internal/polymarket/polymarket.go` | Full Start/Stop implementation |
| `internal/store/queries/tokens.sql` | Added `GetTokenIDsForPlatform` |
| `internal/store/store.go` | Fixed `New()` call |
| `db/migrations/000001_init.up.sql` | Added unique constraint |
| `Dockerfile` | Fixed migrate ENTRYPOINT |
| `Justfile` | Added migrate examples |
| `cmd/collector/main.go` | Cleaned up, uses polymarket.Start() |
