-- name: GetMarketPair :one
SELECT * FROM market_pairs WHERE market_id_a = $1 AND market_id_b = $2;

-- name: GetEquivalentMarkets :many
SELECT * FROM market_pairs
WHERE (market_id_a = $1 OR market_id_b = $1) AND is_equivalent = true;

-- name: UpsertMarketPair :exec
INSERT INTO market_pairs (market_id_a, market_id_b, is_equivalent, confidence, verified_by, llm_model, llm_reasoning, verified_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
ON CONFLICT (market_id_a, market_id_b) DO UPDATE SET
    is_equivalent = EXCLUDED.is_equivalent,
    confidence = EXCLUDED.confidence,
    verified_by = EXCLUDED.verified_by,
    llm_model = EXCLUDED.llm_model,
    llm_reasoning = EXCLUDED.llm_reasoning,
    verified_at = NOW();

-- name: ListUnverifiedPairs :many
SELECT * FROM market_pairs
WHERE verified_by = 'embedding'
ORDER BY confidence DESC
LIMIT $1;

-- name: DeleteMarketPair :exec
DELETE FROM market_pairs WHERE market_id_a = $1 AND market_id_b = $2;
