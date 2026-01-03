-- name: InsertTrade :exec
INSERT INTO trades (time, token_id, trade_id, price, size, side, maker, taker)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: InsertTradeBatch :copyfrom
INSERT INTO trades (time, token_id, trade_id, price, size, side, maker, taker)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetTradeByID :one
SELECT * FROM trades WHERE trade_id = $1;

-- name: GetTradesByToken :many
SELECT * FROM trades
WHERE token_id = $1
ORDER BY time DESC
LIMIT $2;

-- name: GetTradesRange :many
SELECT * FROM trades
WHERE token_id = $1 AND time >= $2 AND time <= $3
ORDER BY time DESC;
