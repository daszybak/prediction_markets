-- Trades table (market trades from the exchange)
-- Prices/sizes stored as BIGINT with scale 10^6 (e.g., 0.75 = 750000)
CREATE TABLE IF NOT EXISTS trades (
    time            TIMESTAMPTZ NOT NULL,
    token_id        TEXT NOT NULL,
    trade_id        TEXT,               -- exchange trade ID if available
    price           BIGINT NOT NULL,    -- scale 10^6
    size            BIGINT NOT NULL,    -- scale 10^6
    side            TEXT NOT NULL,      -- 'buy' or 'sell' (taker side)
    maker           TEXT,               -- maker address if available
    taker           TEXT                -- taker address if available
);

-- Convert to hypertable
SELECT create_hypertable('trades', 'time');

-- Indexes
CREATE INDEX idx_trades_token_time ON trades(token_id, time DESC);
CREATE INDEX idx_trades_trade_id ON trades(trade_id) WHERE trade_id IS NOT NULL;

-- Enable compression after 7 days
ALTER TABLE trades SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'token_id',
    timescaledb.compress_orderby = 'time DESC'
);

SELECT add_compression_policy('trades', INTERVAL '7 days');

-- Retention: keep trades for 1 year
SELECT add_retention_policy('trades', INTERVAL '365 days');
