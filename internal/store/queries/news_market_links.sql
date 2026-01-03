-- name: GetNewsMarketLink :one
SELECT * FROM news_market_links WHERE news_id = $1 AND market_id = $2;

-- name: GetLinksForNews :many
SELECT * FROM news_market_links
WHERE news_id = $1
ORDER BY similarity_score DESC;

-- name: GetLinksForMarket :many
SELECT * FROM news_market_links
WHERE market_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: ListUnanalyzedLinks :many
SELECT * FROM news_market_links
WHERE llm_analyzed = false
ORDER BY similarity_score DESC
LIMIT $1;

-- name: UpsertNewsMarketLink :exec
INSERT INTO news_market_links (news_id, market_id, similarity_score, llm_analyzed, direction, impact_magnitude, llm_confidence, llm_reasoning, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
ON CONFLICT (news_id, market_id) DO UPDATE SET
    similarity_score = EXCLUDED.similarity_score,
    llm_analyzed = EXCLUDED.llm_analyzed,
    direction = EXCLUDED.direction,
    impact_magnitude = EXCLUDED.impact_magnitude,
    llm_confidence = EXCLUDED.llm_confidence,
    llm_reasoning = EXCLUDED.llm_reasoning;

-- name: DeleteNewsMarketLink :exec
DELETE FROM news_market_links WHERE news_id = $1 AND market_id = $2;
