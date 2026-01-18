-- name: InsertOrderBookSnapshot :exec
INSERT INTO order_book_snapshots (event_time, token_id, side, level, price, size, ingested_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: InsertOrderBookSnapshotBatch :copyfrom
INSERT INTO order_book_snapshots (event_time, token_id, side, level, price, size, ingested_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetLatestOrderBookSnapshot :many
SELECT * FROM order_book_snapshots obs
WHERE obs.token_id = $1
AND obs.event_time = (SELECT MAX(sub.event_time) FROM order_book_snapshots sub WHERE sub.token_id = $1)
ORDER BY obs.side, obs.level;

-- name: InsertOrderBookMetrics :exec
INSERT INTO order_book_metrics (
    event_time, token_id, mid_price, best_bid, best_ask, spread, spread_bps,
    bid_depth_5, ask_depth_5, bid_depth_10, ask_depth_10, imbalance, ingested_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);

-- name: InsertOrderBookMetricsBatch :copyfrom
INSERT INTO order_book_metrics (
    event_time, token_id, mid_price, best_bid, best_ask, spread, spread_bps,
    bid_depth_5, ask_depth_5, bid_depth_10, ask_depth_10, imbalance, ingested_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);

-- name: GetLatestOrderBookMetrics :one
SELECT * FROM order_book_metrics
WHERE token_id = $1
ORDER BY event_time DESC
LIMIT 1;

-- name: GetOrderBookMetricsRange :many
SELECT * FROM order_book_metrics
WHERE token_id = $1 AND event_time >= $2 AND event_time <= $3
ORDER BY event_time DESC;
