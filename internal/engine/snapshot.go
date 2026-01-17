package engine

import (
	"context"
	"log/slog"
	"time"

	"github.com/daszybak/prediction_markets/internal/store"
)

// SnapshotWriter periodically captures orderbook state and writes to the database.
type SnapshotWriter struct {
	engine   *Client
	store    *store.Store
	interval time.Duration
	depth    int
	logger   *slog.Logger
}

// NewSnapshotWriter creates a new snapshot writer.
func NewSnapshotWriter(engine *Client, s *store.Store, interval time.Duration, depth int, logger *slog.Logger) *SnapshotWriter {
	return &SnapshotWriter{
		engine:   engine,
		store:    s,
		interval: interval,
		depth:    depth,
		logger:   logger.With("component", "snapshot_writer"),
	}
}

// Start runs the snapshot writer until the context is cancelled.
func (sw *SnapshotWriter) Start(ctx context.Context) {
	ticker := time.NewTicker(sw.interval)
	defer ticker.Stop()

	sw.logger.Info("started snapshot writer", "interval", sw.interval, "depth", sw.depth)

	for {
		select {
		case <-ctx.Done():
			sw.logger.Info("snapshot writer stopped", "error", ctx.Err())
			return
		case <-ticker.C:
			sw.writeSnapshots(ctx)
		}
	}
}

func (sw *SnapshotWriter) writeSnapshots(ctx context.Context) {
	snapshots := sw.engine.TakeSnapshots(sw.depth)
	if len(snapshots) == 0 {
		return
	}

	now := time.Now()
	var params []store.InsertOrderBookSnapshotBatchParams

	for _, snap := range snapshots {
		for level, bid := range snap.Bids {
			// Use level's UpdatedAt as event time, fall back to now if not set.
			eventTime := bid.UpdatedAt
			if eventTime.IsZero() {
				eventTime = now
			}
			params = append(params, store.InsertOrderBookSnapshotBatchParams{
				Time:    eventTime, // Event time from source API
				TokenID: snap.TokenID,
				Side:    "bid",
				Level:   int16(level),
				Price:   int64(bid.Price),
				Size:    int64(bid.Size),
				// ingested_at uses DB default NOW()
			})
		}
		for level, ask := range snap.Asks {
			eventTime := ask.UpdatedAt
			if eventTime.IsZero() {
				eventTime = now
			}
			params = append(params, store.InsertOrderBookSnapshotBatchParams{
				Time:    eventTime, // Event time from source API
				TokenID: snap.TokenID,
				Side:    "ask",
				Level:   int16(level),
				Price:   int64(ask.Price),
				Size:    int64(ask.Size),
				// ingested_at uses DB default NOW()
			})
		}
	}

	if len(params) == 0 {
		return
	}

	count, err := sw.store.InsertOrderBookSnapshotBatch(ctx, params)
	if err != nil {
		sw.logger.Error("failed to write snapshots", "error", err)
		return
	}

	sw.logger.Debug("wrote snapshots", "tokens", len(snapshots), "rows", count)
}
