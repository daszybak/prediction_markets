-- Enable pgvector extension for semantic search
CREATE EXTENSION IF NOT EXISTS vector;

-- Market embeddings for semantic similarity search
CREATE TABLE IF NOT EXISTS market_embeddings (
    market_id               TEXT PRIMARY KEY REFERENCES markets(id) ON DELETE CASCADE,
    description_embedding   vector(384),    -- sentence-transformers/all-MiniLM-L6-v2
    model_name              TEXT NOT NULL DEFAULT 'all-MiniLM-L6-v2',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- HNSW index for fast similarity search (better than IVFFlat for <1M vectors)
CREATE INDEX idx_market_embeddings_description
ON market_embeddings USING hnsw (description_embedding vector_cosine_ops);

-- Cached market pair mappings (LLM-verified equivalence)
CREATE TABLE IF NOT EXISTS market_pairs (
    market_id_a     TEXT NOT NULL REFERENCES markets(id) ON DELETE CASCADE,
    market_id_b     TEXT NOT NULL REFERENCES markets(id) ON DELETE CASCADE,
    is_equivalent   BOOLEAN NOT NULL,
    confidence      FLOAT NOT NULL,
    verified_by     TEXT NOT NULL,          -- 'llm', 'human', 'embedding'
    llm_model       TEXT,                   -- which model verified (e.g., 'claude-3-haiku')
    llm_reasoning   TEXT,                   -- why LLM said yes/no
    verified_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (market_id_a, market_id_b),
    -- Ensure consistent ordering (smaller id first) to avoid duplicates
    CONSTRAINT market_pairs_ordered CHECK (market_id_a < market_id_b)
);

-- Fast lookup for equivalent pairs
CREATE INDEX idx_market_pairs_equivalent
ON market_pairs(market_id_a) WHERE is_equivalent = true;

CREATE INDEX idx_market_pairs_equivalent_b
ON market_pairs(market_id_b) WHERE is_equivalent = true;

-- News articles and their embeddings
CREATE TABLE IF NOT EXISTS news_articles (
    id                  SERIAL PRIMARY KEY,
    source              TEXT NOT NULL,              -- 'reuters', 'twitter', 'bloomberg'
    source_tier         TEXT NOT NULL DEFAULT 'tier3', -- 'tier1', 'tier2', 'tier3'
    url                 TEXT UNIQUE,
    headline            TEXT NOT NULL,
    content             TEXT,
    headline_embedding  vector(384),
    published_at        TIMESTAMPTZ NOT NULL,
    processed_at        TIMESTAMPTZ,                -- when we analyzed it
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_news_articles_published ON news_articles(published_at DESC);
CREATE INDEX idx_news_articles_source ON news_articles(source, published_at DESC);

-- HNSW index for news headline similarity
CREATE INDEX idx_news_articles_embedding
ON news_articles USING hnsw (headline_embedding vector_cosine_ops);

-- Link news to affected markets (many-to-many with analysis results)
CREATE TABLE IF NOT EXISTS news_market_links (
    news_id             INT NOT NULL REFERENCES news_articles(id) ON DELETE CASCADE,
    market_id           TEXT NOT NULL REFERENCES markets(id) ON DELETE CASCADE,
    similarity_score    FLOAT NOT NULL,             -- embedding similarity
    llm_analyzed        BOOLEAN NOT NULL DEFAULT false,
    direction           TEXT,                       -- 'YES_UP', 'YES_DOWN', 'NEUTRAL'
    impact_magnitude    TEXT,                       -- 'small', 'medium', 'large'
    llm_confidence      FLOAT,
    llm_reasoning       TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (news_id, market_id)
);

CREATE INDEX idx_news_market_links_market ON news_market_links(market_id, created_at DESC);
