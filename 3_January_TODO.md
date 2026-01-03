# Architecture Review TODO - January 3, 2025

## Critical (Fix Before Production)

### 1. Race Condition in WebSocket ReadMessage
**File:** `internal/polymarket/websocket/client.go:113-132`

**Problem:** `msg` and `err` variables shared across goroutines without synchronization.

```go
// Current (buggy):
var msg []byte
var err error
go func() {
    _, msg, err = c.conn.ReadMessage()  // Writes without sync
    close(done)
}()
// ... later reads msg, err without sync
```

**Fix:** Use a buffered channel with a result struct:
```go
type readResult struct {
    msg []byte
    err error
}
resultCh := make(chan readResult, 1)
go func() {
    _, msg, err := c.conn.ReadMessage()
    resultCh <- readResult{msg, err}
}()
```

---

### 2. Goroutine Leak in Main Loop
**File:** `cmd/collector/main.go:106-112`

**Problem:** Each `ReadMessage()` call spawns a new goroutine. At high message rates, creates thousands of goroutines that never get cleaned up.

**Fix:** Refactor to reuse a single reader goroutine with a channel, or add proper cleanup.

---

### 3. Fatal Error on Any WebSocket Failure
**File:** `cmd/collector/main.go:109`

**Problem:** `log.Fatalf` crashes entire service on any network error.

```go
// Current:
if err != nil {
    log.Fatalf("Couldn't read message: %v", err)
}

// Should be:
if err != nil {
    log.Printf("websocket error: %v, reconnecting...", err)
    // Implement reconnection with exponential backoff
}
```

---

### 4. Silent Error in Pagination
**File:** `internal/polymarket/clob/client.go:87`

**Problem:** Base64 decode error silently ignored.

```go
// Current:
decodedNextCursor, _ := base64.StdEncoding.DecodeString(*page.NextCursor)

// Should be:
decodedNextCursor, err := base64.StdEncoding.DecodeString(*page.NextCursor)
if err != nil {
    return nil, fmt.Errorf("invalid cursor encoding: %w", err)
}
```

---

### 5. Add Config Validation
**File:** `cmd/collector/main.go`

**Problem:** Missing config fields cause runtime crashes instead of startup failures.

```go
func (c *Config) Validate() error {
    if c.Database.Host == "" {
        return errors.New("database.host is required")
    }
    if c.Database.Port <= 0 || c.Database.Port > 65535 {
        return errors.New("database.port must be 1-65535")
    }
    // ... validate all required fields
    return nil
}
```

---

### 6. Implement Graceful Shutdown
**File:** `cmd/collector/main.go`

**Problem:** No cleanup on shutdown - DB connections and websockets left dangling.

See "Graceful Shutdown Pattern" section below.

---

## High Priority (Before Scaling)

### 7. Implement Kalshi Client
**Files:**
- `internal/kalshi/websocket/client.go` (empty)
- `internal/kalshi/api/client.go:35-36` (unimplemented)

---

### 8. Add Structured Logging
Replace `log` package with `log/slog` for JSON-structured logs.

---

### 9. Add Health Check Endpoint
HTTP endpoint returning service health for Docker/K8s probes.

---

### 10. Extract Service Interfaces
Create interfaces for testability and mocking.

---

### 11. Add Integration Tests
Test actual platform connections, database operations.

---

## Medium Priority (Before Multi-Service)

### 12. Add Circuit Breaker Pattern
See "Circuit Breaker Pattern" section below.

### 13. Add Message Queue (Optional)
See "Message Queue Discussion" section below.

### 14. Add Prometheus Metrics
- Messages processed per second
- Error rates by platform
- WebSocket connection state
- Database query latency

### 15. Cache Market List
Avoid re-fetching entire market list on restart.

---

## Low Priority (Polish)

- [ ] Request rate limiting
- [ ] Request correlation IDs
- [ ] Polygon/wallet integration
- [ ] Trading execution service (arbiter)
- [ ] Semantic search for market matching

---

# Reference Patterns

## Graceful Shutdown Pattern

```go
func main() {
    ctx, cancel := signal.NotifyContext(context.Background(),
        syscall.SIGINT, syscall.SIGTERM)
    defer cancel()

    // Initialize resources
    db := store.New(pool)
    ws, _ := websocket.New(ctx, url)

    // Run main loop in goroutine
    errCh := make(chan error, 1)
    go func() {
        errCh <- runCollector(ctx, db, ws)
    }()

    // Wait for shutdown signal or error
    select {
    case <-ctx.Done():
        log.Println("Shutdown signal received")
    case err := <-errCh:
        log.Printf("Collector error: %v", err)
        cancel()  // Signal other goroutines to stop
    }

    // Graceful shutdown with timeout
    shutdownCtx, shutdownCancel := context.WithTimeout(
        context.Background(), 30*time.Second)
    defer shutdownCancel()

    // Close resources in reverse order of creation
    log.Println("Closing websocket...")
    if err := ws.Close(shutdownCtx); err != nil {
        log.Printf("websocket close error: %v", err)
    }

    log.Println("Closing database...")
    db.Close()

    log.Println("Shutdown complete")
}

func runCollector(ctx context.Context, db *store.Store, ws *websocket.Client) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()  // Clean exit on shutdown
        default:
            msg, err := ws.ReadMessage(ctx)
            if err != nil {
                if ctx.Err() != nil {
                    return nil  // Context cancelled, clean exit
                }
                return fmt.Errorf("read error: %w", err)  // Actual error
            }
            // Process message...
        }
    }
}
```

---

## Circuit Breaker Pattern

**What it is:** A pattern that prevents cascading failures by "tripping" after repeated errors, giving the failing service time to recover.

**States:**
```
CLOSED (normal) → errors exceed threshold → OPEN (failing)
     ↑                                           │
     │                                           ↓
     └──── success ←──── HALF-OPEN (testing) ←── timeout
```

**Implementation:**
```go
type CircuitBreaker struct {
    mu           sync.Mutex
    state        State  // Closed, Open, HalfOpen
    failures     int
    threshold    int           // e.g., 5 failures
    timeout      time.Duration // e.g., 30 seconds
    lastFailure  time.Time
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
    cb.mu.Lock()

    // If circuit is OPEN, check if timeout elapsed
    if cb.state == Open {
        if time.Since(cb.lastFailure) > cb.timeout {
            cb.state = HalfOpen  // Allow one test request
        } else {
            cb.mu.Unlock()
            return ErrCircuitOpen  // Fail fast, don't even try
        }
    }
    cb.mu.Unlock()

    // Execute the actual function
    err := fn()

    cb.mu.Lock()
    defer cb.mu.Unlock()

    if err != nil {
        cb.failures++
        cb.lastFailure = time.Now()
        if cb.failures >= cb.threshold {
            cb.state = Open
        }
        return err
    }

    // Success - reset circuit
    cb.failures = 0
    cb.state = Closed
    return nil
}
```

**Use case in your project:**
```go
polymarketCircuit := NewCircuitBreaker(5, 30*time.Second)

for {
    err := polymarketCircuit.Execute(func() error {
        return ws.ReadMessage(ctx)
    })
    if errors.Is(err, ErrCircuitOpen) {
        log.Println("Polymarket circuit open, waiting...")
        time.Sleep(5 * time.Second)
        continue
    }
    // process...
}
```

---

## Message Queue Discussion

**Do you need it?** Not necessarily for v1, but consider it when:

1. **Processing is slow** - If analyzing/storing a message takes longer than the rate messages arrive, you need buffering
2. **Multiple consumers** - If arbiter service also needs the same market data
3. **Replay capability** - If you want to replay historical data for backtesting

**What you'd process:**
- Order book updates → calculate spreads, detect arbitrage opportunities
- Trade events → update position tracking, trigger alerts
- Market metadata → match markets across platforms, calculate correlations

**Without queue (current):**
```
WebSocket → Process → Store (synchronous, blocks on slow DB)
```

**With queue:**
```
WebSocket → Redis/NATS → Worker 1 (store to DB)
                       → Worker 2 (calculate arbitrage)
                       → Worker 3 (update metrics)
```

**Recommendation:** Start without a queue. Add Redis Streams or NATS if you hit bottlenecks.

---

## Shell Parameter Expansion

**What does `: "${VAR:?message}"` do?**

The colon `:` is a shell builtin that does nothing (no-op) but evaluates its arguments.

```bash
: "${POSTGRES_DB:?POSTGRES_DB is required}"
```

Breaking it down:
- `:` - Do nothing, but evaluate what follows
- `${VAR:?msg}` - If VAR is unset or empty, print "msg" to stderr and exit with code 1

**Why the colon?**
Without it, the shell would try to execute the variable's value as a command:
```bash
"${POSTGRES_DB:?required}"  # If POSTGRES_DB=prediction, runs "prediction" as command!
: "${POSTGRES_DB:?required}"  # Safe - just validates, doesn't execute
```

**Other useful patterns:**
```bash
${VAR:-default}   # Use "default" if VAR unset/empty
${VAR:=default}   # Set VAR to "default" if unset/empty
${VAR:+alternate} # Use "alternate" if VAR is set
${VAR:?error}     # Exit with "error" if VAR unset/empty
```

---

## Market Size Limiting

**Should you limit ingested market size?**

Yes, for several reasons:

1. **Memory protection** - Unbounded `append()` in `GetAllMarkets()` can OOM
2. **Startup time** - Fetching 10,000 markets on every restart is slow
3. **Focus** - You probably only care about liquid markets

**Options:**

```yaml
# configs/collector/config.sample.yaml
platforms:
  polymarket:
    max_markets: 1000           # Hard limit
    min_liquidity: 10000        # Only markets with >$10k volume
    categories:                 # Filter by category
      - politics
      - sports
```

```go
// In clob client
func (c *Client) GetAllMarkets(opts MarketFilterOpts) ([]*Market, error) {
    var markets []*Market
    for {
        if opts.MaxMarkets > 0 && len(markets) >= opts.MaxMarkets {
            break  // Hit limit
        }
        // ... pagination logic
    }
    return markets, nil
}
```

**Recommendation:** Add `max_markets` config with a sensible default (e.g., 500). You can always increase it.
