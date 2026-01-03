-- name: GetToken :one
SELECT * FROM tokens WHERE id = $1;

-- name: GetTokensByMarket :many
SELECT * FROM tokens WHERE market_id = $1 ORDER BY outcome;

-- name: UpsertToken :exec
INSERT INTO tokens (id, market_id, outcome, winning, settlement_price, created_at)
VALUES ($1, $2, $3, $4, $5, NOW())
ON CONFLICT (id) DO UPDATE SET
    outcome = EXCLUDED.outcome,
    winning = EXCLUDED.winning,
    settlement_price = EXCLUDED.settlement_price;

-- name: SetTokenResolution :exec
UPDATE tokens SET winning = $2, settlement_price = $3 WHERE id = $1;

-- name: DeleteToken :exec
DELETE FROM tokens WHERE id = $1;
