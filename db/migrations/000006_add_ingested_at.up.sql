-- Add ingested_at column to track when data was stored
-- `time` = event time (when data was generated at source API)
-- `ingested_at` = ingestion time (when we stored it)
-- Latency = ingested_at - time

-- Order book snapshots
ALTER TABLE order_book_snapshots
ADD COLUMN ingested_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Trades
ALTER TABLE trades
ADD COLUMN ingested_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Order book metrics
ALTER TABLE order_book_metrics
ADD COLUMN ingested_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Add comments for clarity
COMMENT ON COLUMN order_book_snapshots.time IS 'Event time from source API';
COMMENT ON COLUMN order_book_snapshots.ingested_at IS 'When data was stored in our DB';

COMMENT ON COLUMN trades.time IS 'Event time from source API';
COMMENT ON COLUMN trades.ingested_at IS 'When data was stored in our DB';

COMMENT ON COLUMN order_book_metrics.time IS 'Event time from source API';
COMMENT ON COLUMN order_book_metrics.ingested_at IS 'When data was stored in our DB';
