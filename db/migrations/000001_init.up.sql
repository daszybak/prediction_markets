-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Markets table (reference data)
CREATE TABLE IF NOT EXISTS markets (
    id              TEXT PRIMARY KEY,
    platform        TEXT NOT NULL,  -- 'polymarket', 'kalshi'
    description     TEXT NOT NULL,
    end_date        TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_markets_platform ON markets(platform);
CREATE INDEX idx_markets_end_date ON markets(end_date);

-- Tokens table (outcome tokens for each market)
-- Prices stored as BIGINT with scale 10^6 (e.g., 0.75 = 750000)
CREATE TABLE IF NOT EXISTS tokens (
    id                  TEXT PRIMARY KEY,
    market_id           TEXT NOT NULL REFERENCES markets(id),
    outcome             TEXT NOT NULL,      -- 'YES', 'NO', or custom outcome
    winning             BOOLEAN,            -- NULL until resolved, then TRUE/FALSE
    settlement_price    BIGINT,             -- 0 or 1000000 (scale 10^6)
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(market_id, outcome)
);
