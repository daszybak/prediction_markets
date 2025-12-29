-- Order book snapshots (raw depth data)
-- Prices/sizes stored as BIGINT with scale 10^6 (e.g., 0.75 = 750000)
CREATE TABLE IF NOT EXISTS order_book_snapshots (
    time        TIMESTAMPTZ NOT NULL,
    token_id    TEXT NOT NULL,
    side        TEXT NOT NULL,      -- 'bid' or 'ask'
    level       SMALLINT NOT NULL,  -- 0-9 for top 10 levels
    price       BIGINT NOT NULL,    -- scale 10^6
    size        BIGINT NOT NULL     -- scale 10^6
);

-- Convert to hypertable
SELECT create_hypertable('order_book_snapshots', 'time');

-- Indexes for common queries
CREATE INDEX idx_obs_token_time ON order_book_snapshots(token_id, time DESC);
CREATE INDEX idx_obs_token_side_time ON order_book_snapshots(token_id, side, time DESC);

-- Enable compression after 7 days
ALTER TABLE order_book_snapshots SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'token_id, side',
    timescaledb.compress_orderby = 'time DESC, level'
);

SELECT add_compression_policy('order_book_snapshots', INTERVAL '7 days');

-- Retention policy: drop data older than 90 days
SELECT add_retention_policy('order_book_snapshots', INTERVAL '90 days');
