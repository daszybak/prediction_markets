-- Add ingested_at column to track when data was received from WebSocket
-- `event_time` = when event occurred at source API (Polymarket timestamp)
-- `ingested_at` = when we received the data from WebSocket
-- Latency = ingested_at - event_time

-- Note: TimescaleDB hypertables with columnstore don't support DEFAULT NOW()
-- The application must provide ingested_at explicitly on insert.

-- Order book snapshots
ALTER TABLE order_book_snapshots
ADD COLUMN ingested_at TIMESTAMPTZ;

-- Trades
ALTER TABLE trades
ADD COLUMN ingested_at TIMESTAMPTZ;

-- Order book metrics
ALTER TABLE order_book_metrics
ADD COLUMN ingested_at TIMESTAMPTZ;

-- Add comments for clarity
COMMENT ON COLUMN order_book_snapshots.event_time IS 'When event occurred at source API';
COMMENT ON COLUMN order_book_snapshots.ingested_at IS 'When data was received from WebSocket';

COMMENT ON COLUMN trades.event_time IS 'When event occurred at source API';
COMMENT ON COLUMN trades.ingested_at IS 'When data was received from WebSocket';

COMMENT ON COLUMN order_book_metrics.event_time IS 'When event occurred at source API';
COMMENT ON COLUMN order_book_metrics.ingested_at IS 'When data was received from WebSocket';
