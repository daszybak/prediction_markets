# Performance Optimization Notes

Reference for future optimizations. Apply when profiling shows bottlenecks.

---

## JSON Parsing (WebSocket Messages)

**Current:** Two-pass with `encoding/json` (Option 1 - simple)

**When to optimize:** Message processing > 1ms or high CPU in JSON parsing

### Fast JSON Libraries

```go
// github.com/bytedance/sonic - fastest, SIMD-accelerated
import "github.com/bytedance/sonic"
sonic.Unmarshal(msg, &event)

// github.com/goccy/go-json - fast, drop-in replacement
import gojson "github.com/goccy/go-json"
gojson.Unmarshal(msg, &event)

// github.com/tidwall/gjson - fast field extraction without full parse
import "github.com/tidwall/gjson"
eventType := gjson.GetBytes(msg, "event_type").String()
```

### Zero-Allocation with sync.Pool

```go
var bookPool = sync.Pool{
    New: func() any { return &BookEvent{} },
}

func handleBook(msg []byte) {
    book := bookPool.Get().(*BookEvent)
    defer func() {
        *book = BookEvent{}  // reset
        bookPool.Put(book)
    }()

    sonic.Unmarshal(msg, book)
    process(book)
}
```

---

## Channel Architecture (Fan-out)

**Current:** Single goroutine processes all messages

**When to optimize:** Message backpressure, uneven processing times

```go
// Separate channels per message type
bookCh := make(chan *BookEvent, 1000)
priceCh := make(chan *PriceChangeEvent, 1000)

// Router goroutine
go func() {
    for msg := range rawMessages {
        switch getEventType(msg) {
        case "book":
            bookCh <- parseBook(msg)
        case "price_change":
            priceCh <- parsePrice(msg)
        }
    }
}()

// Dedicated handlers (can scale independently)
go processBooks(bookCh)
go processPrices(priceCh)
```

---

## Memory Allocation

### Pre-allocated Slices

```go
// Bad - grows and reallocates
var orders []Order
for _, o := range data {
    orders = append(orders, o)
}

// Good - pre-allocate capacity
orders := make([]Order, 0, len(data))
for _, o := range data {
    orders = append(orders, o)
}
```

### Reuse Buffers

```go
var bufPool = sync.Pool{
    New: func() any {
        b := make([]byte, 0, 4096)
        return &b
    },
}

func process() {
    buf := bufPool.Get().(*[]byte)
    defer bufPool.Put(buf)

    *buf = (*buf)[:0]  // reset length, keep capacity
    // use buf...
}
```

---

## Database

### Batch Inserts

```go
// Bad - N round trips
for _, trade := range trades {
    db.InsertTrade(ctx, trade)
}

// Good - single round trip
db.CopyFrom(ctx, pgx.Identifier{"trades"}, columns, pgx.CopyFromSlice(...))
```

### Connection Pool Tuning

```go
config.MaxConns = 20                    // match expected concurrency
config.MinConns = 5                     // keep warm connections
config.MaxConnLifetime = time.Hour
config.MaxConnIdleTime = 30 * time.Minute
```

---

## Profiling Commands

```bash
# CPU profile
go test -cpuprofile=cpu.prof -bench .
go tool pprof cpu.prof

# Memory profile
go test -memprofile=mem.prof -bench .
go tool pprof mem.prof

# Live profiling (add to main)
import _ "net/http/pprof"
go http.ListenAndServe(":6060", nil)
# Then: go tool pprof http://localhost:6060/debug/pprof/profile
```

---

## Benchmarks

Always benchmark before and after:

```go
func BenchmarkJSONParse(b *testing.B) {
    msg := []byte(`{"event_type":"book",...}`)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        var event BookEvent
        json.Unmarshal(msg, &event)
    }
}
```

Run: `go test -bench=. -benchmem ./...`
