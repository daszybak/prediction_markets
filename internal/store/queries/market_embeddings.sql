-- name: GetMarketEmbedding :one
SELECT * FROM market_embeddings WHERE market_id = $1;

-- name: UpsertMarketEmbedding :exec
INSERT INTO market_embeddings (market_id, description_embedding, model_name)
VALUES ($1, $2, $3)
ON CONFLICT (market_id) DO UPDATE SET
    description_embedding = EXCLUDED.description_embedding,
    model_name = EXCLUDED.model_name,
    updated_at = NOW();

-- name: FindSimilarMarketsByDescription :many
SELECT market_id, description_embedding <=> $1 AS distance
FROM market_embeddings
ORDER BY distance
LIMIT $2;

-- name: DeleteMarketEmbedding :exec
DELETE FROM market_embeddings WHERE market_id = $1;
