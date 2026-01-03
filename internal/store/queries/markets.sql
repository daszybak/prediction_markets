-- name: GetMarket :one
SELECT * FROM markets WHERE id = $1;

-- name: GetMarketsByPlatform :many
SELECT * FROM markets WHERE platform = $1 ORDER BY created_at DESC;

-- name: ListMarkets :many
SELECT * FROM markets ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: UpsertMarket :exec
INSERT INTO markets (id, platform, description, end_date, created_at, updated_at)
VALUES ($1, $2, $3, $4, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
    description = EXCLUDED.description,
    end_date = EXCLUDED.end_date,
    updated_at = NOW();

-- name: DeleteMarket :exec
DELETE FROM markets WHERE id = $1;
