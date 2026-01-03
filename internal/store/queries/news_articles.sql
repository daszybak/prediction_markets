-- name: GetNewsArticle :one
SELECT * FROM news_articles WHERE id = $1;

-- name: GetNewsArticleByURL :one
SELECT * FROM news_articles WHERE url = $1;

-- name: ListRecentNewsArticles :many
SELECT * FROM news_articles
ORDER BY published_at DESC
LIMIT $1 OFFSET $2;

-- name: ListUnprocessedNewsArticles :many
SELECT * FROM news_articles
WHERE processed_at IS NULL
ORDER BY published_at DESC
LIMIT $1;

-- name: InsertNewsArticle :one
INSERT INTO news_articles (source, source_tier, url, headline, content, headline_embedding, published_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
RETURNING id;

-- name: MarkNewsArticleProcessed :exec
UPDATE news_articles SET processed_at = NOW() WHERE id = $1;

-- name: FindSimilarNewsByHeadline :many
SELECT id, headline, headline_embedding <=> $1 AS distance
FROM news_articles
WHERE headline_embedding IS NOT NULL
ORDER BY distance
LIMIT $2;

-- name: DeleteNewsArticle :exec
DELETE FROM news_articles WHERE id = $1;
