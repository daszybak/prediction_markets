-- name: InsertOrderBookSnapshot :exec
INSERT INTO order_book_snapshots (time, token_id, side, level, price, size)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: InsertOrderBookSnapshotBatch :copyfrom
INSERT INTO order_book_snapshots (time, token_id, side, level, price, size)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetLatestOrderBookSnapshot :many
SELECT * FROM order_book_snapshots obs
WHERE obs.token_id = $1
AND obs.time = (SELECT MAX(sub.time) FROM order_book_snapshots sub WHERE sub.token_id = $1)
ORDER BY obs.side, obs.level;

-- name: InsertOrderBookMetrics :exec
INSERT INTO order_book_metrics (
    time, token_id, mid_price, best_bid, best_ask, spread, spread_bps,
    bid_depth_5, ask_depth_5, bid_depth_10, ask_depth_10, imbalance
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);

-- name: InsertOrderBookMetricsBatch :copyfrom
INSERT INTO order_book_metrics (
    time, token_id, mid_price, best_bid, best_ask, spread, spread_bps,
    bid_depth_5, ask_depth_5, bid_depth_10, ask_depth_10, imbalance
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);

-- name: GetLatestOrderBookMetrics :one
SELECT * FROM order_book_metrics
WHERE token_id = $1
ORDER BY time DESC
LIMIT 1;

-- name: GetOrderBookMetricsRange :many
SELECT * FROM order_book_metrics
WHERE token_id = $1 AND time >= $2 AND time <= $3
ORDER BY time DESC;
