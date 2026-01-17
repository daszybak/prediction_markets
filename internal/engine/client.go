// Package engine tracks the order book for a token (market + outcome).
package engine

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/daszybak/prediction_markets/internal/engine/orderbook"
	"github.com/daszybak/prediction_markets/internal/price"
)

const maximumUpdates = 100

type Client struct {
	// tokenid:orderbook_worker
	orderbookWorkers map[string]*OrderbookWorker
	mu               sync.RWMutex
	updates          chan Update
	logger           *slog.Logger
}

type OrderbookWorker struct {
	ob *orderbook.Orderbook
	updates chan Update
	logger *slog.Logger
}

type Update struct {
	Price     price.Price
	Size      price.Size
	TokenID   string
	Side      string
	EventTime time.Time // Timestamp from source API (zero = use current time)
	IsDelta   bool      // true = delta update, false = absolute set
}

type Level struct {
	price price.Price
	size  int64
}

func New(l *slog.Logger) *Client {
	return &Client{
		logger:           l.With("component", "engine"),
		orderbookWorkers: make(map[string]*OrderbookWorker),
		updates:          make(chan Update, maximumUpdates),
	}
}

// Send queues an update for processing. Returns false if the buffer is full.
func (c *Client) Send(u Update) bool {
	select {
	case c.updates <- u:
		return true
	default:
		c.logger.Warn("engine buffer full, dropping update", "token", u.TokenID)
		return false
	}
}

func (obw *OrderbookWorker) start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			obw.logger.Info("context stopped engine", "error", ctx.Err())
			return
		case update := <-obw.updates:
			// Use event time from source, fall back to now if not provided.
			eventTime := update.EventTime
			if eventTime.IsZero() {
				eventTime = time.Now()
			}

			if update.IsDelta {
				obw.ob.Update(update.Price, update.Size, update.Side, eventTime)
			} else {
				obw.ob.Set(update.Price, update.Size, update.Side, eventTime)
			}
		}
	}
}

func (c *Client) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("context stopped engine", "error", ctx.Err())
			return
		case update := <-c.updates:
			c.mu.RLock()
			worker, ok := c.orderbookWorkers[update.TokenID]
			c.mu.RUnlock()

			if !ok {
				c.mu.Lock()
				// Double-check after acquiring write lock.
				worker, ok = c.orderbookWorkers[update.TokenID]
				if !ok {
					worker = &OrderbookWorker{
						ob:      orderbook.New(),
						updates: make(chan Update, maximumUpdates),
						logger:  c.logger.With("tokenID", update.TokenID),
					}
					c.orderbookWorkers[update.TokenID] = worker
					go worker.start(ctx)
				}
				c.mu.Unlock()
			}

			select {
			case worker.updates <- update:
				// Sent.
			default:
				c.logger.Warn("worker buffer full", "token", update.TokenID)
			}
		}
	}
}

// Snapshot captures the current state of an orderbook for a token.
type Snapshot struct {
	TokenID string
	Bids    []orderbook.Level
	Asks    []orderbook.Level
}

// TakeSnapshots returns a snapshot of the top N levels for all active orderbooks.
// This is safe to call concurrently with updates.
func (c *Client) TakeSnapshots(depth int) []Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	snapshots := make([]Snapshot, 0, len(c.orderbookWorkers))
	for tokenID, worker := range c.orderbookWorkers {
		bids, _ := worker.ob.GetTopN("bids", depth)
		asks, _ := worker.ob.GetTopN("asks", depth)
		snapshots = append(snapshots, Snapshot{
			TokenID: tokenID,
			Bids:    bids,
			Asks:    asks,
		})
	}
	return snapshots
}
