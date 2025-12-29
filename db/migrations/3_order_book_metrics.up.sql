-- Order book metrics (aggregated, more useful for analysis)
-- Prices/sizes stored as BIGINT with scale 10^6 (e.g., 0.75 = 750000)
CREATE TABLE IF NOT EXISTS order_book_metrics (
    time            TIMESTAMPTZ NOT NULL,
    token_id        TEXT NOT NULL,
    mid_price       BIGINT,             -- scale 10^6
    best_bid        BIGINT,             -- scale 10^6
    best_ask        BIGINT,             -- scale 10^6
    spread          BIGINT,             -- scale 10^6
    spread_bps      SMALLINT,           -- basis points (integer, e.g., 50 = 0.5%)
    bid_depth_5     BIGINT,             -- scale 10^6
    ask_depth_5     BIGINT,             -- scale 10^6
    bid_depth_10    BIGINT,             -- scale 10^6
    ask_depth_10    BIGINT,             -- scale 10^6
    imbalance       SMALLINT            -- -10000 to 10000 (scale 10^4, e.g., 5000 = 0.5)
);

-- Convert to hypertable
SELECT create_hypertable('order_book_metrics', 'time');

-- Indexes
CREATE INDEX idx_obm_token_time ON order_book_metrics(token_id, time DESC);

-- Enable compression after 7 days
ALTER TABLE order_book_metrics SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'token_id',
    timescaledb.compress_orderby = 'time DESC'
);

SELECT add_compression_policy('order_book_metrics', INTERVAL '7 days');

-- Retention: keep metrics longer than raw snapshots (180 days)
SELECT add_retention_policy('order_book_metrics', INTERVAL '180 days');
