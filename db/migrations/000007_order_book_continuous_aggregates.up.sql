-- Continuous aggregates for order_book_snapshots
-- Pre-computes OHLC + volume rollups at different time intervals

-- 1 minute aggregates
CREATE MATERIALIZED VIEW order_book_snapshots_1m
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 minute', event_time) AS bucket,
    token_id,
    side,
    first(price, event_time) AS open_price,
    max(price) AS high_price,
    min(price) AS low_price,
    last(price, event_time) AS close_price,
    sum(size) AS total_size,
    count(*) AS num_updates
FROM order_book_snapshots
GROUP BY bucket, token_id, side
WITH NO DATA;

-- 5 minute aggregates
CREATE MATERIALIZED VIEW order_book_snapshots_5m
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('5 minutes', event_time) AS bucket,
    token_id,
    side,
    first(price, event_time) AS open_price,
    max(price) AS high_price,
    min(price) AS low_price,
    last(price, event_time) AS close_price,
    sum(size) AS total_size,
    count(*) AS num_updates
FROM order_book_snapshots
GROUP BY bucket, token_id, side
WITH NO DATA;

-- 1 hour aggregates
CREATE MATERIALIZED VIEW order_book_snapshots_1h
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', event_time) AS bucket,
    token_id,
    side,
    first(price, event_time) AS open_price,
    max(price) AS high_price,
    min(price) AS low_price,
    last(price, event_time) AS close_price,
    sum(size) AS total_size,
    count(*) AS num_updates
FROM order_book_snapshots
GROUP BY bucket, token_id, side
WITH NO DATA;

-- 1 day aggregates
CREATE MATERIALIZED VIEW order_book_snapshots_1d
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', event_time) AS bucket,
    token_id,
    side,
    first(price, event_time) AS open_price,
    max(price) AS high_price,
    min(price) AS low_price,
    last(price, event_time) AS close_price,
    sum(size) AS total_size,
    count(*) AS num_updates
FROM order_book_snapshots
GROUP BY bucket, token_id, side
WITH NO DATA;

-- Refresh policies
-- 1m: refresh every 1 minute, process data older than 1 minute, look back 10 minutes
SELECT add_continuous_aggregate_policy('order_book_snapshots_1m',
    start_offset => INTERVAL '10 minutes',
    end_offset => INTERVAL '1 minute',
    schedule_interval => INTERVAL '1 minute'
);

-- 5m: refresh every 5 minutes
SELECT add_continuous_aggregate_policy('order_book_snapshots_5m',
    start_offset => INTERVAL '30 minutes',
    end_offset => INTERVAL '5 minutes',
    schedule_interval => INTERVAL '5 minutes'
);

-- 1h: refresh every 1 hour
SELECT add_continuous_aggregate_policy('order_book_snapshots_1h',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour'
);

-- 1d: refresh every 1 day
SELECT add_continuous_aggregate_policy('order_book_snapshots_1d',
    start_offset => INTERVAL '3 days',
    end_offset => INTERVAL '1 day',
    schedule_interval => INTERVAL '1 day'
);

-- Enable real-time aggregation (combines materialized + recent unmaterialized data)
ALTER MATERIALIZED VIEW order_book_snapshots_1m SET (timescaledb.materialized_only = false);
ALTER MATERIALIZED VIEW order_book_snapshots_5m SET (timescaledb.materialized_only = false);
ALTER MATERIALIZED VIEW order_book_snapshots_1h SET (timescaledb.materialized_only = false);
ALTER MATERIALIZED VIEW order_book_snapshots_1d SET (timescaledb.materialized_only = false);
