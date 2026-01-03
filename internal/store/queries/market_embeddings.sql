-- name: GetMarketEmbedding :one
SELECT * FROM market_embeddings WHERE market_id = $1;

-- name: UpsertMarketEmbedding :exec
INSERT INTO market_embeddings (market_id, question_embedding, description_embedding, model_name, created_at, updated_at)
VALUES ($1, $2, $3, $4, NOW(), NOW())
ON CONFLICT (market_id) DO UPDATE SET
    question_embedding = EXCLUDED.question_embedding,
    description_embedding = EXCLUDED.description_embedding,
    model_name = EXCLUDED.model_name,
    updated_at = NOW();

-- name: FindSimilarMarketsByQuestion :many
SELECT market_id, question_embedding <=> $1 AS distance
FROM market_embeddings
ORDER BY distance
LIMIT $2;

-- name: DeleteMarketEmbedding :exec
DELETE FROM market_embeddings WHERE market_id = $1;
