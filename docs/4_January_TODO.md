# TODO - January 4, 2025

## Completed

- [x] Fix race condition in WebSocket ReadMessage (using result channel)
- [x] Basic graceful shutdown with signal handling in main.go
- [x] Non-fatal error handling in main loop (log.Printf instead of log.Fatalf)
- [x] Goroutine cleanup on context cancel (SetReadDeadline trick)
- [x] Handle base64 decode error in pagination (clob/client.go:90-93)

---

## Critical (Fix Before Production)

### 1. Add Config Validation
**File:** `cmd/collector/main.go`

Missing config fields cause runtime crashes instead of startup failures.

---

## High Priority - Platform Interface Implementation

### 2. Create Platform Interface
**New file:** `internal/platform/platform.go`

```go
type Platform interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Health() HealthStatus
    GetMarkets(ctx context.Context, opts MarketFilterOpts) ([]*Market, error)
    SubscribeOrderBook(ctx context.Context, tokens []string) (<-chan OrderBookUpdate, error)
}
```

### 3. Implement Polymarket Platform
**File:** `internal/polymarket/polymarket.go` (currently empty stub)

---

## Medium Priority

- [ ] Implement Kalshi client
- [ ] Add structured logging (log/slog)
- [ ] Add health check HTTP endpoint
- [ ] Add Prometheus metrics
- [ ] Cache market list to avoid re-fetch on restart
- [ ] Circuit breaker pattern for external calls

---

## Low Priority

- [ ] Request rate limiting
- [ ] Request correlation IDs
- [ ] Trading execution service (arbiter)

---

# Start/Stop Lifecycle Explained

## What Start() Does

`Start(ctx context.Context) error` initializes the platform and spawns background goroutines:

```go
func (p *Polymarket) Start(ctx context.Context) error {
    // 1. Connect to websocket
    ws, err := websocket.New(ctx, p.wsURL)
    if err != nil {
        return fmt.Errorf("websocket connect: %w", err)
    }
    p.ws = ws

    // 2. Initial market fetch
    markets, err := p.clobClient.GetAllMarkets()
    if err != nil {
        return fmt.Errorf("fetch markets: %w", err)
    }
    p.markets = markets

    // 3. Start background goroutines
    p.wg.Add(2)

    // Goroutine 1: Periodic market refresh
    go p.marketRefreshLoop(ctx)

    // Goroutine 2: WebSocket message reader
    go p.wsReaderLoop(ctx)

    return nil
}
```

### Market Refresh Goroutine

Periodically fetches new markets at an interval:

```go
func (p *Polymarket) marketRefreshLoop(ctx context.Context) {
    defer p.wg.Done()

    ticker := time.NewTicker(5 * time.Minute)  // Configurable interval
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return  // Clean exit on shutdown
        case <-ticker.C:
            markets, err := p.clobClient.GetAllMarkets()
            if err != nil {
                log.Printf("market refresh failed: %v", err)
                continue  // Don't crash, try again next interval
            }

            // Find new markets and subscribe to them
            newTokens := p.findNewTokens(markets)
            if len(newTokens) > 0 {
                p.ws.SubscribeMarket(ctx, newTokens, false, nil)
            }

            p.mu.Lock()
            p.markets = markets
            p.mu.Unlock()
        }
    }
}
```

### WebSocket Reader Goroutine

Single long-lived goroutine that reads messages:

```go
func (p *Polymarket) wsReaderLoop(ctx context.Context) {
    defer p.wg.Done()

    for {
        select {
        case <-ctx.Done():
            return
        default:
            msg, err := p.ws.ReadMessage(ctx)
            if err != nil {
                if ctx.Err() != nil {
                    return  // Context cancelled, clean exit
                }
                log.Printf("read error: %v, reconnecting...", err)
                p.reconnect(ctx)  // Implement reconnection with backoff
                continue
            }

            // Send to channel for consumers
            select {
            case p.updates <- msg:
            default:
                log.Printf("update channel full, dropping message")
            }
        }
    }
}
```

## What Stop() Does

`Stop(ctx context.Context) error` cleanly shuts down all goroutines:

```go
func (p *Polymarket) Stop(ctx context.Context) error {
    // 1. Cancel the context passed to Start() - this signals all goroutines to exit
    //    (In practice, the parent context is cancelled, we just wait here)

    // 2. Wait for goroutines with timeout
    done := make(chan struct{})
    go func() {
        p.wg.Wait()  // Wait for marketRefreshLoop and wsReaderLoop to finish
        close(done)
    }()

    select {
    case <-done:
        log.Println("all goroutines stopped cleanly")
    case <-ctx.Done():
        log.Println("shutdown timeout, forcing close")
    }

    // 3. Close connections
    if p.ws != nil {
        if err := p.ws.Close(ctx); err != nil {
            return fmt.Errorf("websocket close: %w", err)
        }
    }

    return nil
}
```

## Full Flow in main.go

```go
func main() {
    ctx, cancel := signal.NotifyContext(context.Background(),
        syscall.SIGINT, syscall.SIGTERM)
    defer cancel()

    // Initialize
    polymarket := polymarket.New(cfg, store)

    // Start spawns the background goroutines
    if err := polymarket.Start(ctx); err != nil {
        log.Fatalf("start polymarket: %v", err)
    }

    // Block until shutdown signal
    <-ctx.Done()
    log.Println("Shutdown signal received")

    // Stop waits for goroutines and cleans up
    shutdownCtx, shutdownCancel := context.WithTimeout(
        context.Background(), 30*time.Second)
    defer shutdownCancel()

    if err := polymarket.Stop(shutdownCtx); err != nil {
        log.Printf("shutdown error: %v", err)
    }

    log.Println("Shutdown complete")
}
```

## Key Points

1. **Start()** spawns goroutines that run until context is cancelled
2. **Stop()** waits for those goroutines to finish (via `sync.WaitGroup`)
3. Context cancellation is the signal for goroutines to exit gracefully
4. `select { case <-ctx.Done(): return }` is the pattern inside goroutines to check for shutdown
5. The market refresh runs on an interval (ticker), not continuously
6. The WebSocket reader runs continuously but checks context between reads
