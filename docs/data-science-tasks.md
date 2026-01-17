# Data Science Tasks

## Project Overview

This project collects real-time data from prediction markets (Polymarket, Kalshi, PredictIt, Metaculus, etc.) and stores it in TimescaleDB. The goal is to identify arbitrage opportunities, match equivalent markets across platforms, and correlate news/events with market movements.

## Architecture

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                           DATA SOURCES                                        │
├─────────────┬─────────────┬─────────────┬─────────────┬─────────────────────┤
│ Polymarket  │   Kalshi    │  PredictIt  │  Metaculus  │   News Feeds        │
│ (WebSocket) │   (API)     │   (API)     │   (API)     │  (RSS/APIs)         │
└──────┬──────┴──────┬──────┴──────┬──────┴──────┬──────┴──────────┬──────────┘
       │             │             │             │                 │
       └─────────────┴─────────────┼─────────────┴─────────────────┘
                                   │
                    ┌──────────────▼──────────────┐
                    │    Go Collector Service     │
                    │                             │
                    │  ┌───────────────────────┐  │
                    │  │   Engine (in-memory)  │  │
                    │  │  goroutine per token  │  │
                    │  │  btree orderbooks     │  │
                    │  │  nanosecond updates   │  │
                    │  └───────────┬───────────┘  │
                    │              │              │
                    │       SnapshotWriter       │
                    │     (periodic batches)     │
                    └──────────────┼──────────────┘
                                   │
       ┌───────────────────────────┼───────────────────────────┐
       │                           │                           │
┌──────▼──────┐                    │                    ┌──────▼──────┐
│ TimescaleDB │◀───────────────────┘                    │  pgvector   │
│(time-series)│                                         │ (embeddings)│
│ snapshots,  │                                         │  market     │
│ aggregates  │                                         │  matching   │
└──────┬──────┘                                         └──────┬──────┘
       │                                                       │
       └───────────────────────┬───────────────────────────────┘
                               │
                    ┌──────────▼──────────┐
                    │   Python Analysis   │
                    │  - Market matching  │
                    │  - Signal detection │
                    │  - Backtesting      │
                    └─────────────────────┘
```

## Supported Platforms

| Platform | Type | Data Available | Status |
|----------|------|----------------|--------|
| **Polymarket** | DeFi/Crypto | WebSocket orderbook (full message parsing), REST market sync | ✅ Active |
| **Kalshi** | US Regulated | REST API client started, WebSocket pending | 🚧 In Progress |
| **PredictIt** | US Political | REST API | Planned |
| **Metaculus** | Community/Scientific | REST API | Planned |
| **Manifold** | Play Money | REST API | Planned |
| **Betfair** | Sports/Politics | REST API | Planned |
| **Augur** | Decentralized | Blockchain events | Planned |

## Related Projects

Before building, check these existing projects for inspiration:

| Project | Description | Relevant For |
|---------|-------------|--------------|
| [Polymarket/agents](https://github.com/Polymarket/agents) | Official AI agents with Chroma vectorization | News vectorization, market metadata |
| [PredictOS](https://github.com/PredictionXBT/PredictOS) | Opensource prediction market framework | Architecture patterns |
| [awesome-prediction-markets](https://github.com/0xperp/awesome-prediction-markets) | Curated list of tools | Discovery |
| [Awesome-Prediction-Market-Tools](https://github.com/aarora4/Awesome-Prediction-Market-Tools) | AI agents, analytics, APIs | Tool ecosystem |
| [polymarket-kalshi-arbitrage-bot](https://github.com/terauss/Polymarket-Kalshi-Arbitrage-bot) | Cross-platform arbitrage | Smart matching implementation |

**Key insight from Polymarket/agents**: They use Chroma for vectorizing news sources and have a `GammaMarketClient` for fetching market metadata - similar to our approach but with a different vector DB.

## Vector Database (pgvector)

We use **pgvector** (PostgreSQL extension) for semantic search and market correlation:

### Use Cases

1. **Market Matching Across Platforms**
   - Embed market questions using sentence transformers
   - Find equivalent markets: "Will Bitcoin hit $100k?" ≈ "BTC >= $100,000 EOY"
   - Enables cross-platform arbitrage detection

2. **News → Market Correlation**
   - Embed news headlines/articles
   - Find which markets a news item affects
   - Early signal detection before price moves

3. **Similar Market Discovery**
   - "Show me markets similar to X"
   - Portfolio correlation analysis
   - Risk clustering

### Schema Extension

```sql
-- Enable pgvector
CREATE EXTENSION IF NOT EXISTS vector;

-- Market embeddings for semantic search
CREATE TABLE market_embeddings (
    market_id TEXT PRIMARY KEY REFERENCES markets(id),
    description_embedding vector(384),  -- sentence-transformers/all-MiniLM-L6-v2
    model_name TEXT NOT NULL DEFAULT 'all-MiniLM-L6-v2',
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- HNSW index for fast similarity search (better than IVFFlat for <1M vectors)
CREATE INDEX ON market_embeddings
USING hnsw (description_embedding vector_cosine_ops);

-- News articles and their embeddings
CREATE TABLE news_articles (
    id SERIAL PRIMARY KEY,
    source TEXT NOT NULL,           -- reuters, bloomberg, twitter, etc.
    url TEXT UNIQUE,
    headline TEXT NOT NULL,
    content TEXT,
    published_at TIMESTAMPTZ NOT NULL,
    headline_embedding vector(384),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX ON news_articles
USING hnsw (headline_embedding vector_cosine_ops);

-- Link news to affected markets
CREATE TABLE news_market_links (
    news_id INT REFERENCES news_articles(id),
    market_id TEXT REFERENCES markets(id),
    similarity_score FLOAT,
    PRIMARY KEY (news_id, market_id)
);
```

### Example Queries

```python
from sentence_transformers import SentenceTransformer
import numpy as np

model = SentenceTransformer('all-MiniLM-L6-v2')

# Find similar markets
def find_similar_markets(question: str, top_k: int = 5):
    embedding = model.encode(question)

    return pd.read_sql("""
        SELECT m.*, me.description_embedding <=> %s::vector AS distance
        FROM markets m
        JOIN market_embeddings me ON m.id = me.market_id
        ORDER BY distance
        LIMIT %s
    """, engine, params=[embedding.tolist(), top_k])

# Match news to markets
def find_affected_markets(headline: str, threshold: float = 0.3):
    embedding = model.encode(headline)

    return pd.read_sql("""
        SELECT m.*, 1 - (me.description_embedding <=> %s::vector) AS similarity
        FROM markets m
        JOIN market_embeddings me ON m.id = me.market_id
        WHERE 1 - (me.description_embedding <=> %s::vector) > %s
        ORDER BY similarity DESC
    """, engine, params=[embedding.tolist(), embedding.tolist(), threshold])
```

### Embedding Limitations

**IMPORTANT**: Embeddings capture topic similarity, NOT sentiment/outcome:

```python
# DANGER: These have ~0.87 similarity but OPPOSITE meanings!
a = "Trump wins the 2024 election"
b = "Trump loses the 2024 election"
```

Use embeddings for candidate retrieval, then verify with LLM or rules.

## Market Matching Strategy (Hybrid Approach)

### The Core Problem

Market rules correlation has two distinct dimensions:

| Dimension | What It Means | Example |
|-----------|---------------|---------|
| **Semantic similarity** | Topics/entities match | "Will Bitcoin exceed $100k?" ≈ "BTC price above 100000 USD?" |
| **Resolution equivalence** | Same outcome under ALL scenarios | Same event + same criteria + same timeframe + same source |

**Critical insight**: Embeddings excel at semantic similarity but struggle with resolution equivalence because subtle rule differences can change outcomes:
- "Bitcoin > $100k on Dec 31, 2025 at 23:59 UTC"
- "Bitcoin > $100k anytime in December 2025"

These embed ~95% similarly but resolve differently.

### Why Hybrid?

| Method | Speed | Cost | Accuracy |
|--------|-------|------|----------|
| Embeddings only | <50ms | Free | 60-70% |
| LLM only | 1-3s | $0.01-0.05/call | 90%+ |
| **Hybrid** | <50ms after cache | One-time LLM cost | 90%+ |

### Pipeline

```
New Market Appears on Platform B
           │
           ▼
┌─────────────────────────────┐
│  Stage 1: Embedding Filter  │  Fast, free
│  Find top-5 candidates from │
│  Platform A with sim > 0.6  │
└─────────────┬───────────────┘
              │ 5 candidates
              ▼
┌─────────────────────────────┐
│  Stage 2: LLM Verification  │  One-time cost ~$0.02
│  "Are these the same bet?"  │
│  Returns: yes/no + confidence│
└─────────────┬───────────────┘
              │
              ▼
┌─────────────────────────────┐
│  Stage 3: Cache Forever     │  market_pairs table
│  Never ask LLM again        │
└─────────────────────────────┘
```

### Schema

```sql
-- Cached market pair mappings (LLM-verified)
CREATE TABLE market_pairs (
    market_id_a TEXT REFERENCES markets(id),
    market_id_b TEXT REFERENCES markets(id),
    is_equivalent BOOLEAN NOT NULL,
    confidence FLOAT NOT NULL,
    verified_by TEXT NOT NULL,        -- 'llm' | 'human' | 'embedding'
    llm_reasoning TEXT,               -- Why LLM said yes/no
    verified_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (market_id_a, market_id_b)
);

-- Index for fast lookups
CREATE INDEX idx_market_pairs_equivalent
ON market_pairs(market_id_a) WHERE is_equivalent = true;
```

### Implementation

```python
async def find_equivalent_market(new_market: Market) -> Optional[Market]:
    """Find equivalent market across platforms, using cache."""

    # 1. Check cache first
    cached = await db.fetch_one("""
        SELECT market_id_b, confidence
        FROM market_pairs
        WHERE market_id_a = $1 AND is_equivalent = true
    """, new_market.id)

    if cached:
        return await get_market(cached.market_id_b)

    # 2. Embedding similarity for candidates
    embedding = model.encode(new_market.question)
    candidates = await db.fetch_all("""
        SELECT m.*, 1 - (me.description_embedding <=> $1::vector) as sim
        FROM markets m
        JOIN market_embeddings me ON m.id = me.market_id
        WHERE m.platform != $2
          AND 1 - (me.description_embedding <=> $1::vector) > 0.6
        ORDER BY sim DESC
        LIMIT 5
    """, embedding.tolist(), new_market.platform)

    if not candidates:
        return None

    # 3. LLM verification (one-time cost)
    for candidate in candidates:
        result = await llm_verify_match(new_market, candidate)

        # Cache the result (positive or negative)
        await db.execute("""
            INSERT INTO market_pairs
            (market_id_a, market_id_b, is_equivalent, confidence, verified_by, llm_reasoning)
            VALUES ($1, $2, $3, $4, 'llm', $5)
        """, new_market.id, candidate.id, result.is_match,
             result.confidence, result.reasoning)

        if result.is_match and result.confidence > 0.8:
            return candidate

    return None


async def llm_verify_match(market_a: Market, market_b: Market) -> MatchResult:
    """Use LLM to verify if two markets are equivalent."""

    prompt = f"""
    Are these two prediction markets asking about the SAME event/outcome?

    Market A ({market_a.platform}):
    Question: {market_a.question}
    Resolution: {market_a.resolution_date}

    Market B ({market_b.platform}):
    Question: {market_b.question}
    Resolution: {market_b.resolution_date}

    Consider:
    - Same underlying event?
    - Same resolution criteria?
    - Same time frame?

    Respond in JSON:
    {{
        "is_match": true/false,
        "confidence": 0.0-1.0,
        "reasoning": "brief explanation"
    }}
    """

    # Use cheap model (Haiku/GPT-4o-mini) - $0.01-0.02 per call
    response = await llm.complete(prompt, model="claude-3-haiku")
    return parse_match_result(response)
```

### Cost Analysis

```
Scenario: 1000 markets across 3 platforms

Worst case (all unique):
- 1000 markets × 5 LLM calls each = 5000 calls
- 5000 × $0.01 = $50 one-time cost

Typical case (50% have matches):
- 500 markets find match on first try = 500 calls
- 500 markets check all 5 candidates = 2500 calls
- 3000 × $0.01 = $30 one-time cost

After initial mapping:
- New markets: ~10/day × $0.05 = $0.50/day
- Cached lookups: FREE
```

---

## Advanced Market Correlation Approaches

### Approach 1: Embedding Model Selection

Current setup uses `all-MiniLM-L6-v2` (384 dims). Consider these alternatives:

| Model | Dims | Speed | Quality | Notes |
|-------|------|-------|---------|-------|
| `all-MiniLM-L6-v2` | 384 | ⚡ Fast | Good | Current choice, good baseline |
| `all-mpnet-base-v2` | 768 | Medium | Better | Better semantic understanding |
| `BAAI/bge-small-en-v1.5` | 384 | ⚡ Fast | Better | State-of-art for size, drop-in replacement |
| `nomic-embed-text-v1.5` | 768 | Medium | Excellent | Best open-source quality |
| `text-embedding-3-small` | 1536 | API | Excellent | OpenAI, $0.02/1M tokens |

**Recommendation**: Upgrade to `BAAI/bge-small-en-v1.5` - same dimensions, better quality, free.

### Approach 2: Structured Extraction + Embedding (Recommended)

Extract structured fields BEFORE embedding to normalize the representation:

```
┌──────────────────┐     ┌─────────────────────┐     ┌──────────────────┐
│ Raw Market Rules │────▶│ Rule Parser/LLM     │────▶│ Structured Data  │
└──────────────────┘     └─────────────────────┘     └──────────────────┘
                                                              │
                    ┌─────────────────────────────────────────┤
                    ▼                                         ▼
          ┌──────────────────┐                     ┌──────────────────┐
          │ Canonical Text   │                     │ Field Matching   │
          │ Embedding        │                     │ (Exact/Fuzzy)    │
          └──────────────────┘                     └──────────────────┘
```

**Structured extraction schema:**

```go
type ParsedMarketRules struct {
    Subject          string    `json:"subject"`           // "Bitcoin", "Trump", "S&P 500"
    Metric           string    `json:"metric"`            // "price", "election outcome", "index value"
    Condition        string    `json:"condition"`         // ">", "<", "=", "between"
    Threshold        string    `json:"threshold"`         // "100000", "win", "4500-4600"
    ResolutionTime   time.Time `json:"resolution_time"`   // Exact resolution timestamp
    ResolutionSource string    `json:"resolution_source"` // "CoinGecko", "AP", "official results"
    Outcomes         []string  `json:"outcomes"`          // ["YES", "NO"] or custom
}
```

**SQL schema extension:**

```sql
ALTER TABLE markets ADD COLUMN parsed_rules JSONB;
CREATE INDEX idx_markets_parsed_rules ON markets USING gin(parsed_rules);

-- Example queries:
-- Find all Bitcoin markets
SELECT * FROM markets WHERE parsed_rules->>'subject' = 'Bitcoin';

-- Find markets resolving in same hour
SELECT * FROM markets
WHERE parsed_rules->>'resolution_time' BETWEEN $1 AND $2;
```

**Why this works better:**
1. **Exact field matching**: Two markets with identical `Subject + Metric + Threshold + ResolutionTime` are likely equivalent
2. **Embedding on canonical form**: "Bitcoin price exceeds $100,000 by 2025-12-31" embeds more consistently
3. **Confidence scoring**: Combine field match score + embedding similarity

### Approach 3: Fine-tuned Domain Embedding (Best Long-term)

Train a domain-specific embedding model on your verified market pairs:

```python
from sentence_transformers import SentenceTransformer, InputExample, losses
from torch.utils.data import DataLoader

# Load verified market pairs from database
# SELECT * FROM market_pairs WHERE verified_by IN ('human', 'llm') AND confidence > 0.9

train_examples = [
    # Positive pairs (equivalent markets)
    InputExample(texts=[market_a_rules, market_b_rules], label=1.0),
    # Negative pairs (different markets)
    InputExample(texts=[market_a_rules, market_c_rules], label=0.0),
]

model = SentenceTransformer('BAAI/bge-small-en-v1.5')
train_dataloader = DataLoader(train_examples, shuffle=True, batch_size=16)
train_loss = losses.CosineSimilarityLoss(model)

model.fit(
    train_objectives=[(train_dataloader, train_loss)],
    epochs=10,
    warmup_steps=100,
    output_path='models/market-embeddings-v1'
)
```

**Requirements**: ~500-1000 verified market pairs from `market_pairs` table.

### Approach 4: Small LLM Selection for Verification

For the LLM verification stage, these models provide best cost/quality:

| Model | Type | Latency | Quality | Cost | Best For |
|-------|------|---------|---------|------|----------|
| `Qwen2.5-3B-Instruct` | Local | 50-200ms | Good | Free | High volume, local deployment |
| `Phi-3-mini-4k` | Local | 50-200ms | Good | Free | Resource constrained |
| `Llama-3.2-3B-Instruct` | Local | 50-200ms | Good | Free | General purpose |
| `claude-3-haiku` | API | 200-500ms | Excellent | $0.25/1M in | Production quality |
| `gpt-4o-mini` | API | 200-500ms | Excellent | $0.15/1M in | Production quality |
| `gemini-1.5-flash` | API | 100-300ms | Good | $0.075/1M in | Cheapest API |

**Recommendation for your use case:**
- **Development/Testing**: Local Qwen2.5-3B via Ollama
- **Production**: claude-3-haiku or gpt-4o-mini
- **High volume production**: Self-hosted Qwen2.5-7B on GPU

### Recommended 4-Stage Pipeline

```
                              ┌─────────────────────────────────┐
                              │     New Market Ingested         │
                              └─────────────────┬───────────────┘
                                                │
                                                ▼
                         ┌───────────────────────────────────────────┐
                         │  Stage 1: Structured Extraction           │
                         │  - Small LLM or regex patterns            │
                         │  - Extract: subject, metric, threshold,   │
                         │    resolution_time, source                │
                         │  - Generate canonical description         │
                         │  Cost: ~$0.001/market or free (local)     │
                         └─────────────────────┬─────────────────────┘
                                               │
                                               ▼
                         ┌───────────────────────────────────────────┐
                         │  Stage 2: Candidate Generation            │
                         │  - Embed canonical description            │
                         │  - pgvector similarity search             │
                         │  - Return top-20 candidates               │
                         │  Cost: Free, Latency: <50ms               │
                         └─────────────────────┬─────────────────────┘
                                               │
                                               ▼
                         ┌───────────────────────────────────────────┐
                         │  Stage 3: Field Matching (Fast Filter)    │
                         │  - Exact match: resolution_time ±1 hour   │
                         │  - Fuzzy match: subject similarity >0.9   │
                         │  - Reduce to top-5 candidates             │
                         │  Cost: Free, Latency: <10ms               │
                         └─────────────────────┬─────────────────────┘
                                               │
                                               ▼
                         ┌───────────────────────────────────────────┐
                         │  Stage 4: LLM Verification (Precision)    │
                         │  - Full rules comparison                  │
                         │  - Edge case analysis                     │
                         │  - Output: equivalent, confidence, reason │
                         │  Cost: ~$0.01/comparison                  │
                         └─────────────────────┬─────────────────────┘
                                               │
                                               ▼
                         ┌───────────────────────────────────────────┐
                         │  Stage 5: Store & Feedback                │
                         │  - Cache in market_pairs table            │
                         │  - Use for fine-tuning embeddings         │
                         └───────────────────────────────────────────┘
```

### When to Skip LLM Verification

Pure embedding + field matching is sufficient when:
- Markets have identical `end_date` (within hours)
- Subject extraction matches exactly (same ticker/entity)
- Embedding similarity > 0.95
- You only need recall (finding candidates), not precision

Use LLM verification when:
- Cross-platform matching (Kalshi ↔ Polymarket rules differ in format)
- Embedding similarity between 0.7-0.95 (uncertain zone)
- High-stakes decisions (arbitrage opportunities > $100)
- Building training data for fine-tuned embeddings

## Signal Classification (When to Use LLM)

Not all signals need expensive LLM verification. Use this decision tree:

### High-Stakes Signal Detection

```python
def is_high_stakes(signal: Signal, portfolio: Portfolio) -> bool:
    """Determine if signal warrants LLM verification."""

    # 1. Position exposure - do you have skin in the game?
    position = portfolio.get_position(signal.market_id)
    if position and abs(position.value_usd) > 500:
        return True

    # 2. Arbitrage opportunity size
    if signal.type == "arbitrage" and signal.potential_profit_usd > 100:
        return True

    # 3. Source quality (tier1 = Reuters, AP, Bloomberg, official gov)
    if signal.source_tier == "tier1":
        return True

    # 4. Market liquidity - can you actually trade on this?
    if signal.market_liquidity_usd < 1000:
        return False  # Not worth the LLM cost

    # 5. Cross-platform signal (same news affects multiple platforms)
    if signal.affected_platforms >= 2:
        return True

    return False
```

### Signal Processing Pipeline

```
Incoming Signal (news, price move, etc.)
              │
              ▼
┌─────────────────────────────┐
│  is_high_stakes() check     │
└─────────────┬───────────────┘
              │
       ┌──────┴──────┐
       │             │
    HIGH          LOW
       │             │
       ▼             ▼
┌─────────────┐  ┌─────────────┐
│ LLM Analysis│  │ Rules only  │
│ - Direction │  │ - Log it    │
│ - Confidence│  │ - Maybe alert│
│ - Reasoning │  └─────────────┘
└──────┬──────┘
       │
       ▼
┌─────────────────────────────┐
│  Action Decision            │
│  confidence > 0.7 → trade   │
│  confidence > 0.5 → alert   │
│  else → log only            │
└─────────────────────────────┘
```

### LLM for News → Market Direction

```python
async def analyze_news_impact(news: str, market: Market) -> ImpactAnalysis:
    """Use LLM to determine how news affects a market."""

    prompt = f"""
    News headline: "{news}"

    Prediction market: "{market.question}"
    Current YES price: ${market.yes_price:.2f} ({market.yes_price*100:.0f}% implied probability)

    How does this news affect the market?

    Respond in JSON:
    {{
        "relevant": true/false,
        "direction": "YES_UP" | "YES_DOWN" | "NEUTRAL",
        "magnitude": "small" | "medium" | "large",
        "confidence": 0.0-1.0,
        "reasoning": "brief explanation",
        "suggested_fair_value": 0.0-1.0 or null
    }}
    """

    response = await llm.complete(prompt, model="claude-3-haiku")
    return parse_impact_analysis(response)


# Example usage
async def process_news_signal(news: NewsItem, portfolio: Portfolio):
    # 1. Find affected markets via embedding similarity
    affected_markets = await find_affected_markets(news.headline, threshold=0.4)

    for market in affected_markets:
        signal = Signal(
            type="news",
            market_id=market.id,
            source_tier=news.source_tier,
            market_liquidity_usd=market.daily_volume,
            affected_platforms=len(affected_markets),
        )

        if is_high_stakes(signal, portfolio):
            # Worth the LLM cost
            impact = await analyze_news_impact(news.headline, market)

            if impact.relevant and impact.confidence > 0.7:
                await create_alert(
                    market=market,
                    news=news,
                    direction=impact.direction,
                    confidence=impact.confidence,
                    reasoning=impact.reasoning,
                )
        else:
            # Just log for later analysis
            await log_signal(signal, news)
```

### Cost Control

```python
class LLMBudget:
    """Track and limit LLM spending."""

    def __init__(self, daily_limit_usd: float = 10.0):
        self.daily_limit = daily_limit_usd
        self.spent_today = 0.0
        self.last_reset = datetime.now().date()

    async def can_spend(self, estimated_cost: float) -> bool:
        self._maybe_reset()

        if self.spent_today + estimated_cost > self.daily_limit:
            logger.warning(f"LLM budget exhausted: ${self.spent_today:.2f}/${self.daily_limit:.2f}")
            return False
        return True

    async def record_spend(self, actual_cost: float):
        self.spent_today += actual_cost

    def _maybe_reset(self):
        if datetime.now().date() > self.last_reset:
            self.spent_today = 0.0
            self.last_reset = datetime.now().date()


# Usage
budget = LLMBudget(daily_limit_usd=10.0)

async def analyze_with_budget(news: str, market: Market):
    cost_estimate = 0.02  # ~$0.02 per Haiku call

    if not await budget.can_spend(cost_estimate):
        return None  # Skip LLM, use rules only

    result = await analyze_news_impact(news, market)
    await budget.record_spend(cost_estimate)
    return result
```

## News Feed Integration

### Sources to Consider

| Source | Type | Use Case |
|--------|------|----------|
| **Reuters/AP** | Wire services | Breaking news |
| **Twitter/X** | Social | Fast signals, sentiment |
| **Bloomberg** | Financial | Market-moving news |
| **Official Gov Sources** | Primary | Election results, economic data |
| **Reddit** | Social | Sentiment, early signals |
| **Crypto Twitter** | Specialized | Crypto market signals |

### News Processing Pipeline

```
Raw Feed → Dedup → Embed → Match Markets → Store → Alert
              ↓
         Rate limit &
         source quality
```

```python
# Pseudo-code for news processor
async def process_news_item(article: NewsArticle):
    # 1. Generate embedding
    embedding = model.encode(article.headline)

    # 2. Find affected markets
    affected_markets = find_affected_markets(
        embedding,
        threshold=0.4
    )

    # 3. Store with links
    article_id = store_article(article, embedding)
    for market in affected_markets:
        link_news_to_market(article_id, market.id, market.similarity)

    # 4. Alert if high-impact
    if len(affected_markets) > 0 and article.source_quality > 0.8:
        await send_alert(article, affected_markets)
```

## Data Available

### Markets Table (planned schema)

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Internal ID |
| `source` | TEXT | polymarket, kalshi, coinbase |
| `external_id` | TEXT | Market ID from source |
| `question` | TEXT | Market question/title |
| `description` | TEXT | Full description |
| `outcomes` | JSONB | Possible outcomes |
| `resolution_date` | TIMESTAMPTZ | When market resolves |
| `created_at` | TIMESTAMPTZ | When we first saw it |

### Prices Table (hypertable)

| Column | Type | Description |
|--------|------|-------------|
| `time` | TIMESTAMPTZ | Timestamp (partitioned) |
| `market_id` | UUID | FK to markets |
| `outcome` | TEXT | Which outcome |
| `price` | DECIMAL | Price (0.00-1.00) |
| `volume` | DECIMAL | Trade volume |
| `bid` | DECIMAL | Best bid |
| `ask` | DECIMAL | Best ask |

## Prediction Market Fundamentals

### Basic Terms

| Term | Definition |
|------|------------|
| **Binary market** | Yes/No outcome. Prices represent probability (0.55 = 55% chance of YES). |
| **Multi-outcome market** | Multiple mutually exclusive outcomes (e.g., "Who wins election?" with 5 candidates). |
| **Resolution** | When the market settles. Winning outcome pays $1, losers pay $0. |
| **Implied probability** | The price. A YES at $0.40 implies 40% probability. |
| **Overround** | When probabilities sum to >100%. The excess is the market maker's edge. |
| **Underround** | When probabilities sum to <100%. This is an arbitrage opportunity. |
| **Bid** | Highest price someone will pay (buy order). |
| **Ask** | Lowest price someone will sell at (sell order). |
| **Spread** | Ask minus Bid. Wider spread = less liquid market. |
| **Slippage** | Price moves against you as you execute. Large orders cause more slippage. |
| **Market maker** | Provides liquidity by placing both bid and ask orders, profits from spread. |
| **Taker** | Executes against existing orders (pays the spread). |

### How Prediction Markets Work

```
Market: "Will it rain tomorrow?"

Order Book:
  YES buyers (bids)     YES sellers (asks)
  $0.42 (100 shares)    $0.45 (50 shares)   ← spread = $0.03
  $0.40 (200 shares)    $0.47 (100 shares)
  $0.38 (150 shares)    $0.50 (200 shares)

If you buy 50 YES shares: you pay $0.45 each
If you buy 150 YES shares: you pay $0.45 for 50, then $0.47 for 100 (slippage)

Tomorrow:
  - If rain: YES pays $1.00, NO pays $0.00
  - If no rain: YES pays $0.00, NO pays $1.00
```

### Key Insight

**Price = Probability (in efficient markets)**

If YES trades at $0.60, the market collectively believes there's a 60% chance of YES.

But markets aren't always efficient. That's where opportunities exist.

---

## Pattern Detection Methods

### 1. Arbitrage (Risk-Free Profit)

#### Same-Market Arbitrage

Binary outcomes must sum to 100%. If not, free money:

```python
def find_same_market_arb(yes_price, no_price):
    total = yes_price + no_price
    if total < 1.0:
        # Buy both sides
        profit_pct = (1.0 - total) / total * 100
        return f"Buy YES@{yes_price} + NO@{no_price}, profit {profit_pct:.1f}%"
    return None

# Example: YES=0.45, NO=0.52, total=0.97
# Cost: $0.97, guaranteed payout: $1.00, profit: 3.1%
```

#### Cross-Platform Arbitrage

Same event, different prices:

```python
def find_cross_platform_arb(markets):
    """
    markets = [
        {"source": "polymarket", "yes": 0.42, "no": 0.58},
        {"source": "kalshi", "yes": 0.48, "no": 0.53},
    ]
    """
    # Find cheapest YES and cheapest NO across platforms
    min_yes = min(m["yes"] for m in markets)
    min_no = min(m["no"] for m in markets)

    if min_yes + min_no < 1.0:
        return f"Arb exists: buy YES@{min_yes} on one, NO@{min_no} on other"
    return None
```

**Challenges**:
- Execution risk (prices move while you trade)
- Capital locked until resolution
- Platform fees eat into profit
- Withdrawal delays

### 2. Whale Detection

Large trades often signal informed money.

```python
def detect_whale_trades(df, volume_threshold_pct=95):
    """
    df has columns: time, price, volume
    """
    threshold = df["volume"].quantile(volume_threshold_pct / 100)
    whales = df[df["volume"] > threshold].copy()

    # Calculate price impact
    whales["price_before"] = df["price"].shift(1)
    whales["price_after"] = df["price"].shift(-1)
    whales["impact"] = whales["price_after"] - whales["price_before"]

    return whales

# Questions to answer:
# 1. Do whale buys predict price increases?
# 2. How long does the signal last?
# 3. Is there mean reversion after whale trades?
```

**Signal quality metrics**:
- Hit rate: % of whale trades where price continued in same direction
- Average return: mean price change after whale trade
- Decay: how quickly does predictive power diminish?

### 3. Spread Analysis

Wide spreads = opportunity for market making.

```python
def analyze_spreads(df):
    """
    df has columns: time, bid, ask
    """
    df["spread"] = df["ask"] - df["bid"]
    df["spread_pct"] = df["spread"] / ((df["bid"] + df["ask"]) / 2) * 100

    # Find markets with consistently wide spreads
    avg_spread = df.groupby("market_id")["spread_pct"].mean()
    wide_spread_markets = avg_spread[avg_spread > 5]  # >5% spread

    return wide_spread_markets

# Market making strategy:
# Place bid at (mid - X), ask at (mid + X)
# Profit from spread when both sides fill
# Risk: adverse selection (informed traders pick you off)
```

### 4. Mean Reversion

Prices often overreact to news, then revert.

```python
def find_mean_reversion(df, window=20, threshold=2):
    """
    Look for prices that deviate significantly from moving average.
    """
    df["ma"] = df["price"].rolling(window).mean()
    df["std"] = df["price"].rolling(window).std()
    df["zscore"] = (df["price"] - df["ma"]) / df["std"]

    # Signals
    df["oversold"] = df["zscore"] < -threshold  # price too low
    df["overbought"] = df["zscore"] > threshold  # price too high

    return df

# Backtest: buy when oversold, sell when overbought
# Measure: does price revert to mean? How long? What's the return?
```

### 5. Event-Driven Patterns

News moves markets. Detect the pattern.

```python
def analyze_event_impact(prices_df, events_df):
    """
    prices_df: time, market_id, price
    events_df: time, market_id, event_type (news, tweet, etc.)
    """
    results = []

    for _, event in events_df.iterrows():
        # Get prices around event
        mask = (
            (prices_df["market_id"] == event["market_id"]) &
            (prices_df["time"] >= event["time"] - timedelta(hours=1)) &
            (prices_df["time"] <= event["time"] + timedelta(hours=24))
        )
        window = prices_df[mask].copy()

        if len(window) < 2:
            continue

        # Calculate impact
        pre_price = window[window["time"] < event["time"]]["price"].iloc[-1]
        post_prices = window[window["time"] > event["time"]]["price"]

        results.append({
            "event_type": event["event_type"],
            "immediate_impact": post_prices.iloc[0] - pre_price if len(post_prices) > 0 else None,
            "1h_impact": post_prices.iloc[:12].mean() - pre_price if len(post_prices) >= 12 else None,  # assuming 5min candles
        })

    return pd.DataFrame(results)
```

### 6. Time-of-Day Effects

Markets may behave differently at certain times.

```python
def time_of_day_analysis(df):
    df["hour"] = df["time"].dt.hour
    df["day_of_week"] = df["time"].dt.dayofweek

    # Volatility by hour
    hourly_vol = df.groupby("hour")["price"].std()

    # Spread by hour (liquidity patterns)
    hourly_spread = df.groupby("hour")["spread"].mean()

    # Volume by hour
    hourly_volume = df.groupby("hour")["volume"].sum()

    return hourly_vol, hourly_spread, hourly_volume

# Look for:
# - Low liquidity hours (wider spreads, more slippage)
# - High volatility periods (more opportunities, more risk)
# - Volume patterns (when do whales trade?)
```

---

## Statistical Framework

### Expected Value (EV)

Every trade should have positive EV:

```
EV = (P(win) × win_amount) - (P(lose) × lose_amount) - fees

Example:
  Buy YES at $0.40
  You believe true probability is 50%

  EV = (0.50 × $0.60) - (0.50 × $0.40) - $0.01 fee
  EV = $0.30 - $0.20 - $0.01
  EV = $0.09 per share (positive, good trade)
```

### Kelly Criterion

Optimal position sizing:

```python
def kelly_fraction(win_prob, win_amount, lose_amount):
    """
    Returns fraction of bankroll to bet.
    """
    # Kelly formula: f = (bp - q) / b
    # where b = odds, p = win prob, q = lose prob
    b = win_amount / lose_amount
    p = win_prob
    q = 1 - p

    kelly = (b * p - q) / b

    # Half-Kelly is more conservative
    return max(0, kelly / 2)

# Example: 55% edge, 1:1 payout
# kelly_fraction(0.55, 1, 1) = 0.05 → bet 5% of bankroll
```

### Sharpe Ratio

Risk-adjusted returns:

```python
def sharpe_ratio(returns, risk_free_rate=0.05):
    excess_returns = returns - risk_free_rate / 252  # daily
    return excess_returns.mean() / excess_returns.std() * np.sqrt(252)

# Sharpe > 1: decent
# Sharpe > 2: good
# Sharpe > 3: excellent (or overfitted)
```

### Maximum Drawdown

Worst peak-to-trough loss:

```python
def max_drawdown(equity_curve):
    peak = equity_curve.expanding().max()
    drawdown = (equity_curve - peak) / peak
    return drawdown.min()

# -20% max drawdown means at worst you lost 20% from peak
```

---

## Market Microstructure

Understanding *why* prices move, not just *that* they move.

### The Market Maker's Problem

You're quoting both sides:
```
Your quotes:  BID $0.48  |  ASK $0.52  (spread = $0.04)

Three types of counterparties:
1. Noise traders   - random, you profit from spread
2. Informed traders - they know something, you lose
3. Other MMs       - competing for the same spread
```

**Adverse selection**: Informed traders only trade when your price is wrong. If someone eagerly hits your ask, ask yourself: *what do they know that I don't?*

### Order Flow Toxicity

Not all volume is equal.

```python
def calculate_vpin(trades_df, bucket_size=50):
    """
    Volume-Synchronized Probability of Informed Trading (VPIN).
    High VPIN = more informed/toxic flow.
    """
    df = trades_df.copy()

    # Classify trades as buy/sell (using tick rule or quote rule)
    df["side"] = np.where(
        df["price"] > df["price"].shift(1), "buy",
        np.where(df["price"] < df["price"].shift(1), "sell", "unknown")
    )

    # Bucket by volume, not time
    df["cum_volume"] = df["volume"].cumsum()
    df["bucket"] = (df["cum_volume"] // bucket_size).astype(int)

    # Calculate order imbalance per bucket
    buckets = df.groupby("bucket").agg({
        "volume": "sum",
        "side": lambda x: (x == "buy").sum() - (x == "sell").sum()
    })
    buckets["abs_imbalance"] = buckets["side"].abs()

    # VPIN = average |imbalance| / volume
    vpin = buckets["abs_imbalance"].rolling(50).mean() / bucket_size

    return vpin

# VPIN > 0.4 historically preceded flash crashes
# Use as risk signal: widen spreads or reduce inventory
```

### The Information Hierarchy

```
                    ┌─────────────────┐
                    │  Insider info   │  ← illegal in securities, gray area in prediction markets
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │  Proprietary    │  ← your models, faster data feeds
                    │  research       │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │  Public info    │  ← news, filings, tweets
                    │  (fast)         │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │  Public info    │  ← same info, processed slowly
                    │  (slow)         │
                    └─────────────────┘

You profit by being higher on this ladder than your counterparty.
```

### Price Impact Models

Your order moves the market. Model it.

```python
def estimate_price_impact(order_size, avg_daily_volume, volatility, side="buy"):
    """
    Square-root model (Almgren-Chriss style).
    Impact ≈ σ × sqrt(Q/V)
    """
    participation_rate = order_size / avg_daily_volume
    impact = volatility * np.sqrt(participation_rate)

    return impact if side == "buy" else -impact

# Example:
# Order: 1000 shares, ADV: 100,000, volatility: 2%
# Impact ≈ 0.02 × sqrt(0.01) = 0.2%

# For prediction markets, this is often larger due to low liquidity
```

**Permanent vs Temporary Impact**:
- **Temporary**: Price bounces back after your order (liquidity cost)
- **Permanent**: Price stays moved (you revealed information)

```python
def decompose_impact(trades_df, decay_window=100):
    """
    Separate permanent from temporary impact.
    """
    impacts = []

    for i, trade in trades_df.iterrows():
        if i + decay_window >= len(trades_df):
            break

        immediate_impact = trades_df.iloc[i + 1]["price"] - trade["price"]
        settled_impact = trades_df.iloc[i + decay_window]["price"] - trade["price"]

        impacts.append({
            "trade_size": trade["volume"],
            "immediate": immediate_impact,
            "permanent": settled_impact,
            "temporary": immediate_impact - settled_impact,
        })

    return pd.DataFrame(impacts)

# If permanent >> temporary: you're trading with edge (or leaking info)
# If temporary >> permanent: you're just paying liquidity costs
```

---

## Thinking Like a Market Maker

### The Fundamental Equation

```
P&L = Spread Capture - Adverse Selection - Inventory Risk

Where:
  Spread Capture   = half-spread × number of round-trips
  Adverse Selection = loss to informed traders
  Inventory Risk    = loss from holding positions that move against you
```

### Inventory Management

Holding inventory = taking risk. Manage it.

```python
class MarketMaker:
    def __init__(self, max_inventory=1000, half_life=100):
        self.inventory = 0
        self.max_inventory = max_inventory
        self.half_life = half_life  # trades until inventory decays

    def get_skewed_quotes(self, fair_price, base_spread):
        """
        Skew quotes to reduce inventory.
        If long, lower both bid and ask to attract sellers.
        """
        half_spread = base_spread / 2

        # Inventory skew: shift quotes based on position
        inventory_fraction = self.inventory / self.max_inventory
        skew = inventory_fraction * half_spread  # max skew = half the spread

        bid = fair_price - half_spread - skew
        ask = fair_price + half_spread - skew

        return bid, ask

    def on_fill(self, side, size, price):
        if side == "buy":  # we bought, customer sold
            self.inventory += size
        else:  # we sold, customer bought
            self.inventory -= size

        # Check risk limits
        if abs(self.inventory) > self.max_inventory:
            self.flatten_inventory()

# When inventory is high:
# - Skew quotes to incentivize reducing trades
# - Widen spreads (more risk = more compensation)
# - Consider hedging with correlated markets
```

### Fair Value Estimation

You need an opinion on fair value to quote around.

```python
def estimate_fair_value(orderbook, trades, method="microprice"):
    """
    Multiple approaches to fair value.
    """
    if method == "midpoint":
        # Simple: (bid + ask) / 2
        return (orderbook["best_bid"] + orderbook["best_ask"]) / 2

    elif method == "microprice":
        # Weight by size: more weight to side with less size
        # (less size = more likely to be hit = more informative)
        bid_size = orderbook["bid_size"]
        ask_size = orderbook["ask_size"]
        imbalance = ask_size / (bid_size + ask_size)  # 0 to 1

        return orderbook["best_bid"] * imbalance + orderbook["best_ask"] * (1 - imbalance)

    elif method == "trade_weighted":
        # Recent trades weighted by recency
        recent = trades.tail(20).copy()
        weights = np.exp(-np.arange(len(recent))[::-1] / 5)  # exponential decay
        return np.average(recent["price"], weights=weights)

    elif method == "bayesian":
        # Prior + likelihood from order flow
        # This is where it gets sophisticated...
        pass

# Microprice typically outperforms midpoint
# But the real edge is in your bayesian update from order flow
```

### Quote Optimization

How wide should your spread be?

```python
def optimal_spread(volatility, inventory, time_horizon, risk_aversion=0.1):
    """
    Avellaneda-Stoikov model for optimal quotes.
    """
    # Reservation price (where you're indifferent to trading)
    reservation_price = fair_value - inventory * risk_aversion * volatility**2 * time_horizon

    # Optimal spread
    spread = risk_aversion * volatility**2 * time_horizon + (2 / risk_aversion) * np.log(1 + risk_aversion / k)

    return reservation_price, spread

# Key insight: spread should be wider when:
# - Volatility is high (more risk)
# - Inventory is high (more risk)
# - Time horizon is long (more exposure)
```

---

## Edge: Finding and Sizing

### What is Edge?

Edge = your expected profit per trade, after costs.

```
Edge = (Your estimate of true probability) - (Market implied probability) - (Costs)

Example:
  Market says: 40% chance (YES @ $0.40)
  You believe: 50% chance
  Fees + slippage: 1%

  Theoretical edge: 10%
  Real edge: 10% - 1% = 9%
  EV per $1 risked: $0.09
```

### Edge Decay

Edge disappears as others discover it.

```python
def analyze_edge_decay(signal_df, forward_returns, max_horizon=100):
    """
    How long does your signal predict returns?
    """
    correlations = []

    for horizon in range(1, max_horizon + 1):
        # Correlation between signal and forward return at each horizon
        fwd_ret = forward_returns.shift(-horizon)
        corr = signal_df["signal"].corr(fwd_ret)
        correlations.append({"horizon": horizon, "correlation": corr})

    decay_df = pd.DataFrame(correlations)

    # Find half-life (where correlation drops to 50% of max)
    max_corr = decay_df["correlation"].max()
    half_life = decay_df[decay_df["correlation"] < max_corr / 2]["horizon"].iloc[0]

    return decay_df, half_life

# If half-life = 5 minutes: you need fast execution
# If half-life = 1 day: you can afford slower, larger trades
```

### The Sharpe-Capacity Tradeoff

```
High Sharpe, Low Capacity:
  - Microstructure signals (order flow)
  - Small, illiquid markets
  - Decays in minutes

Medium Sharpe, Medium Capacity:
  - Statistical arbitrage
  - Cross-market relationships
  - Decays in hours to days

Low Sharpe, High Capacity:
  - Fundamental value investing
  - Macro trends
  - Decays in weeks to months
```

Jane Street operates across the spectrum but excels at high-frequency market making where:
- Edge per trade is tiny (fractions of a cent)
- But you do millions of trades
- Speed and technology are the moat

---

## Execution Algorithms

How to get in/out without moving the market.

### TWAP (Time-Weighted Average Price)

```python
def twap_schedule(total_quantity, duration_minutes, interval_minutes=1):
    """
    Spread order evenly over time.
    """
    num_slices = duration_minutes // interval_minutes
    slice_size = total_quantity / num_slices

    schedule = []
    for i in range(num_slices):
        schedule.append({
            "time_offset": i * interval_minutes,
            "quantity": slice_size
        })

    return schedule

# Pros: simple, predictable
# Cons: doesn't adapt to market conditions
```

### VWAP (Volume-Weighted Average Price)

```python
def vwap_schedule(total_quantity, historical_volume_profile):
    """
    Trade more when market is more liquid.
    """
    # historical_volume_profile: Series indexed by time, values = fraction of daily volume
    schedule = []

    for time, volume_fraction in historical_volume_profile.items():
        schedule.append({
            "time": time,
            "quantity": total_quantity * volume_fraction
        })

    return schedule

# Pros: lower market impact
# Cons: predictable (others can front-run)
```

### Implementation Shortfall (IS)

```python
class ISExecutor:
    """
    Minimize: (Execution Price - Arrival Price) × Quantity

    Urgency parameter trades off:
    - Fast execution (more impact, less risk)
    - Slow execution (less impact, more risk)
    """
    def __init__(self, urgency=0.5):
        self.urgency = urgency  # 0 = patient, 1 = aggressive
        self.arrival_price = None
        self.executed_qty = 0

    def on_market_update(self, orderbook, remaining_qty):
        if self.arrival_price is None:
            self.arrival_price = (orderbook["bid"] + orderbook["ask"]) / 2

        # Calculate optimal trade rate
        spread = orderbook["ask"] - orderbook["bid"]
        volatility = self.estimate_volatility()

        # Trade faster if:
        # - High urgency
        # - Low spread (cheap to trade)
        # - High volatility (risky to wait)
        trade_fraction = self.urgency * (1 + volatility / spread)
        trade_qty = min(remaining_qty * trade_fraction, orderbook["ask_size"])

        return trade_qty
```

---

## Risk Management

### Position Limits

```python
class RiskManager:
    def __init__(self):
        self.limits = {
            "per_market": 10000,      # max $ per market
            "per_sector": 50000,      # max $ per correlated group
            "total_gross": 200000,    # max total exposure
            "total_net": 50000,       # max directional exposure
            "var_95": 10000,          # max 95% daily VaR
        }
        self.positions = {}

    def check_order(self, market_id, side, quantity, price):
        proposed_delta = quantity * price * (1 if side == "buy" else -1)

        # Per-market limit
        current = self.positions.get(market_id, 0)
        if abs(current + proposed_delta) > self.limits["per_market"]:
            return False, "per_market limit"

        # Gross exposure
        gross = sum(abs(p) for p in self.positions.values())
        if gross + abs(proposed_delta) > self.limits["total_gross"]:
            return False, "gross limit"

        # Net exposure
        net = sum(self.positions.values())
        if abs(net + proposed_delta) > self.limits["total_net"]:
            return False, "net limit"

        return True, "ok"
```

### Correlation Risk

Positions that look diversified may not be.

```python
def analyze_correlation_risk(positions_df, returns_df):
    """
    Are your positions actually diversified?
    """
    # Calculate correlation matrix
    corr_matrix = returns_df.corr()

    # Portfolio variance (not just sum of individual variances)
    weights = positions_df["weight"].values
    portfolio_var = weights @ corr_matrix.values @ weights

    # Diversification ratio
    sum_of_variances = (positions_df["weight"] * returns_df.std()).sum() ** 2
    diversification_ratio = np.sqrt(sum_of_variances / portfolio_var)

    # > 1 means you have diversification benefit
    # = 1 means perfectly correlated (no benefit)

    return diversification_ratio, corr_matrix

# Example: long "Trump wins" on Polymarket, short "Harris wins"
# These are 100% negatively correlated - it's ONE bet, not two
```

### Greeks for Prediction Markets

Borrow concepts from options:

```python
def calculate_greeks(position, current_price, days_to_resolution, volatility):
    """
    Prediction market "greeks"
    """
    # Delta: sensitivity to price
    # For binary, always 1 (you win $1 if right)
    delta = 1 if position > 0 else -1

    # Theta: time decay
    # As resolution approaches, uncertainty resolves
    # Price moves toward 0 or 1
    theta = estimate_theta(current_price, days_to_resolution)

    # Vega: sensitivity to volatility
    # Higher vol = more uncertainty = prices closer to 0.5
    vega = estimate_vega(current_price, volatility)

    # Gamma: rate of change of delta
    # In binaries, this is the "knife edge" near resolution
    gamma = 0  # delta is constant for binaries

    return {"delta": delta, "theta": theta, "vega": vega, "gamma": gamma}

def estimate_theta(price, days):
    """
    Simplified: price drifts toward extremes as time passes.
    If price > 0.5, theta is positive (price drifts up)
    If price < 0.5, theta is negative (price drifts down)
    """
    drift_per_day = (price - 0.5) / days
    return drift_per_day

# Use these to understand your portfolio's sensitivities
# "I'm long theta" = I profit as time passes (resolution risk)
# "I'm short vol" = I lose if uncertainty increases
```

---

## The Jane Street Mindset

### First Principles

1. **Expected value is everything.** Every decision is an EV calculation.

2. **You don't need to be right, you need to be less wrong than the market.** If market says 40%, you say 42%, and truth is 45%, you still made money.

3. **The market is your counterparty, not your friend.** Every trade has someone on the other side. Why are they trading with you?

4. **Risk is not the enemy, uncompensated risk is.** Take risks you're paid for. Hedge risks you're not.

5. **Speed is a feature.** Not just execution speed, but speed of learning, adapting, deploying.

### Questions to Ask About Every Trade

```
Before entering:
  1. What is my edge? (quantify it)
  2. What is the market missing?
  3. Who is on the other side? Why are they wrong?
  4. What would make me wrong?
  5. How does this correlate with my existing positions?

After the trade:
  1. Was my prediction correct? By how much?
  2. Was my sizing correct?
  3. What did I learn?
  4. How can I incorporate this into my models?
```

### Building Intuition

```python
# Daily exercises:

def daily_calibration():
    """
    Track your probability estimates vs outcomes.
    """
    predictions = load_your_predictions()  # [(event, your_prob, outcome), ...]

    # Brier score: mean squared error of probabilities
    brier = np.mean([(p - o)**2 for _, p, o in predictions])

    # Calibration: when you say 70%, does it happen 70% of the time?
    bins = np.arange(0, 1.1, 0.1)
    for i in range(len(bins) - 1):
        in_bin = [(p, o) for _, p, o in predictions if bins[i] <= p < bins[i+1]]
        if in_bin:
            predicted = np.mean([p for p, _ in in_bin])
            actual = np.mean([o for _, o in in_bin])
            print(f"Predicted {predicted:.0%}, Actual {actual:.0%}")

    return brier

# Good traders have Brier scores < 0.2
# Perfect calibration: predicted % = actual %
```

### Common Mistakes

| Mistake | Reality |
|---------|---------|
| "I was right but unlucky" | Track results, not feelings. Luck averages out. |
| "This market is inefficient" | Maybe. Or maybe you're missing something. |
| "I'll wait for a better price" | Opportunity cost is real. Calculate the EV of waiting. |
| "I should double down" | Only if new information changed your estimate. Losses aren't information. |
| "I need to make back my losses" | Each trade is independent. Past losses are sunk costs. |
| "This is a sure thing" | Nothing is certain. Assign real probabilities. |

---

## Your Tasks

### Phase 1: Market Correlation (pgvector)

**Goal**: Match equivalent markets across platforms using semantic search.

Example:
- Polymarket: "Will Bitcoin hit $100k in 2025?"
- Kalshi: "BTC price >= $100,000 on Dec 31, 2025"

These are the same market but worded differently.

**Approach** (using pgvector):
1. Generate embeddings for all market questions using `sentence-transformers`
2. Store in `market_embeddings` table
3. Use cosine similarity to find matches across platforms
4. Apply threshold + manual review for edge cases

**Deliverable**: Python script/notebook that:
- Connects to TimescaleDB + pgvector
- Generates and stores embeddings for all markets
- Groups equivalent markets across sources
- Outputs mapping table: `(source_a, market_id_a, source_b, market_id_b, similarity_score)`

### Phase 1.5: News Feed Integration

**Goal**: Collect news and correlate with markets for early signal detection.

**Tasks**:
1. Set up news collectors (RSS feeds, Twitter API, etc.)
2. Generate embeddings for headlines
3. Auto-link news to affected markets via similarity
4. Build alerting pipeline for high-impact news

**Deliverable**:
- News collector service (Go or Python)
- Migration for news tables (see schema above)
- Dashboard/alerts for news → market correlations

### Phase 2: Signal Discovery

**Goal**: Find exploitable patterns.

#### 2.1 Arbitrage Detection

Binary markets should sum to ~100%. When they don't:

```
Market: "Will X happen?"
YES: 0.45 (45%)
NO:  0.52 (52%)
Sum: 0.97 → 3% arbitrage opportunity
```

Cross-platform arbitrage:
```
Polymarket YES: 0.42
Kalshi YES:     0.48
→ Buy Polymarket, sell Kalshi (if correlated)
```

#### 2.2 Whale Detection

Large trades that move the market. Look for:
- Sudden volume spikes
- Price impact > threshold
- Wallet/account clustering (if available)

#### 2.3 Pattern Analysis

- Mean reversion after large moves
- Time-of-day effects
- Event-driven patterns (news, tweets)

**Deliverable**: Jupyter notebooks with:
- Signal definitions
- Historical frequency analysis
- Expected value calculations

### Phase 3: Backtesting

**Goal**: Validate strategies on historical data.

**Framework requirements**:
- Realistic execution (slippage, fees)
- Position sizing
- Risk metrics (Sharpe, max drawdown, etc.)

**Suggested structure**:
```
analysis/
├── notebooks/
│   ├── 01_market_correlation.ipynb
│   ├── 02_arbitrage_detection.ipynb
│   ├── 03_whale_signals.ipynb
│   └── 04_backtesting.ipynb
├── src/
│   ├── db.py           # TimescaleDB connection
│   ├── correlation.py  # Market matching
│   ├── signals.py      # Signal generation
│   └── backtest.py     # Backtesting engine
└── requirements.txt
```

## Getting Started

### 1. Setup

```bash
# Start database (includes TimescaleDB with pgvector)
just deps

# Create .env from sample
cp .env.sample .env
# Edit with your credentials

# Install Python deps
pip install -r analysis/requirements.txt
```

**Required Python packages** (add to `analysis/requirements.txt`):
```
sqlalchemy>=2.0
psycopg2-binary
pandas
numpy
sentence-transformers  # for embeddings
pgvector              # pgvector Python client
```

### 2. Enable pgvector

```sql
-- Run once after DB setup (or add to migrations)
CREATE EXTENSION IF NOT EXISTS vector;
```

### 3. Connect to TimescaleDB + pgvector

```python
import os
from sqlalchemy import create_engine
from pgvector.sqlalchemy import Vector

engine = create_engine(os.environ["DATABASE_URL"])
# Or: postgresql://prediction:password@localhost:5432/prediction

# Test pgvector
with engine.connect() as conn:
    result = conn.execute("SELECT '[1,2,3]'::vector")
    print("pgvector working:", result.fetchone())
```

### 4. Example Query

```python
import pandas as pd

# Get price history for a market
df = pd.read_sql("""
    SELECT time, price, volume
    FROM prices
    WHERE market_id = %s
    ORDER BY time
""", engine, params=[market_id])
```

## Questions to Answer

1. **Correlation accuracy**: What's the false positive rate for market matching?
2. **Arbitrage frequency**: How often do opportunities appear? How long do they last?
3. **Whale predictiveness**: Do large trades predict price direction?
4. **Strategy capacity**: How much capital can each strategy deploy?

## Communication

- Data schema changes: coordinate with Go collector
- New data requirements: open an issue
- Strategy results: document in notebooks with clear methodology
