# LLM Service Architecture

## Overview

This document describes the architecture for an LLM service to handle:
1. **Market rule parsing** - Extract structured data from free-form rules text
2. **Market equivalence verification** - Determine if two markets resolve identically
3. **News impact analysis** - Analyze how news affects market prices

## Architecture Decision

### Option A: Go-native LLM Service (Recommended)

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           Go Collector Service                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐                 │
│  │ Polymarket  │    │   Kalshi    │    │   LLM       │                 │
│  │  Platform   │    │  Platform   │    │  Service    │                 │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘                 │
│         │                  │                  │                         │
│         └──────────────────┼──────────────────┘                         │
│                            │                                            │
│                     ┌──────▼──────┐                                     │
│                     │    Store    │                                     │
│                     └─────────────┘                                     │
└─────────────────────────────────────────────────────────────────────────┘
```

**Pros:**
- Single binary deployment
- Shared database connection pool
- Direct access to market data
- No network overhead for internal calls

**Cons:**
- LLM blocking can affect collector
- Need careful goroutine management

### Option B: Separate Python/Go Microservice

```
┌─────────────────┐         ┌─────────────────┐
│  Go Collector   │◀──gRPC──▶│  LLM Service    │
│                 │   or     │  (Go or Python) │
│                 │   HTTP   │                 │
└────────┬────────┘         └────────┬────────┘
         │                           │
         └───────────┬───────────────┘
              ┌──────▼──────┐
              │  Database   │
              └─────────────┘
```

**Pros:**
- Independent scaling
- Can use Python ML ecosystem
- Isolated failure domain

**Cons:**
- Deployment complexity
- Network latency
- Data synchronization

### Recommendation

Start with **Option A** (Go-native) with clean interfaces, allowing extraction to a microservice later if needed.

---

## Go Service Design

### Package Structure

```
internal/
├── llm/
│   ├── llm.go           # Interface definitions
│   ├── client.go        # Client with provider selection
│   ├── providers/
│   │   ├── anthropic.go # Claude API
│   │   ├── openai.go    # OpenAI/GPT API
│   │   └── ollama.go    # Local Ollama
│   ├── prompts/
│   │   ├── market_parse.go
│   │   ├── market_match.go
│   │   └── news_impact.go
│   └── budget.go        # Cost tracking
├── matcher/
│   ├── matcher.go       # Market matching orchestration
│   ├── embedder.go      # Embedding generation
│   └── pipeline.go      # 4-stage pipeline
```

### Core Interfaces

```go
// internal/llm/llm.go
package llm

import (
    "context"
    "time"
)

// Provider represents an LLM backend
type Provider interface {
    // Complete sends a prompt and returns the response
    Complete(ctx context.Context, req *Request) (*Response, error)

    // Name returns the provider name for logging
    Name() string

    // EstimateCost estimates cost for a request (in USD)
    EstimateCost(req *Request) float64
}

// Request represents an LLM completion request
type Request struct {
    Model       string            `json:"model"`
    Messages    []Message         `json:"messages"`
    Temperature float64           `json:"temperature,omitempty"`
    MaxTokens   int               `json:"max_tokens,omitempty"`
    JSONMode    bool              `json:"json_mode,omitempty"`    // Force JSON output
    Metadata    map[string]string `json:"-"`                      // Internal tracking
}

// Message represents a chat message
type Message struct {
    Role    string `json:"role"`    // "system", "user", "assistant"
    Content string `json:"content"`
}

// Response represents an LLM completion response
type Response struct {
    Content      string        `json:"content"`
    Model        string        `json:"model"`
    InputTokens  int           `json:"input_tokens"`
    OutputTokens int           `json:"output_tokens"`
    Latency      time.Duration `json:"latency"`
    Cost         float64       `json:"cost_usd"`
}

// ParsedMarketRules represents extracted market structure
type ParsedMarketRules struct {
    Subject          string    `json:"subject"`
    Metric           string    `json:"metric"`
    Condition        string    `json:"condition"`
    Threshold        string    `json:"threshold"`
    ResolutionTime   time.Time `json:"resolution_time"`
    ResolutionSource string    `json:"resolution_source"`
    Outcomes         []string  `json:"outcomes"`
    Canonical        string    `json:"canonical"` // Normalized description for embedding
}

// MarketMatchResult represents equivalence verification result
type MarketMatchResult struct {
    IsEquivalent bool     `json:"is_equivalent"`
    Confidence   float64  `json:"confidence"`
    Reasoning    string   `json:"reasoning"`
    Differences  []string `json:"differences,omitempty"`
}

// NewsImpactResult represents news analysis result
type NewsImpactResult struct {
    Relevant        bool    `json:"relevant"`
    Direction       string  `json:"direction"` // "YES_UP", "YES_DOWN", "NEUTRAL"
    Magnitude       string  `json:"magnitude"` // "small", "medium", "large"
    Confidence      float64 `json:"confidence"`
    Reasoning       string  `json:"reasoning"`
    SuggestedValue  *float64 `json:"suggested_fair_value,omitempty"`
}
```

### Provider Implementations

```go
// internal/llm/providers/anthropic.go
package providers

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/daszybak/prediction_markets/internal/llm"
)

type AnthropicProvider struct {
    apiKey     string
    httpClient *http.Client
    baseURL    string
}

type AnthropicConfig struct {
    APIKey  string
    Timeout time.Duration
}

func NewAnthropic(cfg AnthropicConfig) *AnthropicProvider {
    return &AnthropicProvider{
        apiKey:  cfg.APIKey,
        baseURL: "https://api.anthropic.com/v1",
        httpClient: &http.Client{
            Timeout: cfg.Timeout,
        },
    }
}

func (p *AnthropicProvider) Name() string {
    return "anthropic"
}

func (p *AnthropicProvider) Complete(ctx context.Context, req *llm.Request) (*llm.Response, error) {
    start := time.Now()

    // Convert to Anthropic format
    anthropicReq := map[string]any{
        "model":      req.Model,
        "max_tokens": req.MaxTokens,
        "messages":   p.convertMessages(req.Messages),
    }

    if req.Temperature > 0 {
        anthropicReq["temperature"] = req.Temperature
    }

    body, _ := json.Marshal(anthropicReq)

    httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(body))
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }

    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("x-api-key", p.apiKey)
    httpReq.Header.Set("anthropic-version", "2023-06-01")

    resp, err := p.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("do request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("anthropic error: status %d", resp.StatusCode)
    }

    var anthropicResp struct {
        Content []struct {
            Text string `json:"text"`
        } `json:"content"`
        Model string `json:"model"`
        Usage struct {
            InputTokens  int `json:"input_tokens"`
            OutputTokens int `json:"output_tokens"`
        } `json:"usage"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
        return nil, fmt.Errorf("decode response: %w", err)
    }

    content := ""
    if len(anthropicResp.Content) > 0 {
        content = anthropicResp.Content[0].Text
    }

    return &llm.Response{
        Content:      content,
        Model:        anthropicResp.Model,
        InputTokens:  anthropicResp.Usage.InputTokens,
        OutputTokens: anthropicResp.Usage.OutputTokens,
        Latency:      time.Since(start),
        Cost:         p.calculateCost(req.Model, anthropicResp.Usage.InputTokens, anthropicResp.Usage.OutputTokens),
    }, nil
}

func (p *AnthropicProvider) convertMessages(msgs []llm.Message) []map[string]string {
    // Anthropic uses system prompt separately, handle conversion
    result := make([]map[string]string, 0, len(msgs))
    for _, m := range msgs {
        if m.Role == "system" {
            continue // Handle separately
        }
        result = append(result, map[string]string{
            "role":    m.Role,
            "content": m.Content,
        })
    }
    return result
}

func (p *AnthropicProvider) EstimateCost(req *llm.Request) float64 {
    // Rough estimate: 4 chars per token
    inputTokens := len(req.Messages[0].Content) / 4
    outputTokens := req.MaxTokens
    return p.calculateCost(req.Model, inputTokens, outputTokens)
}

func (p *AnthropicProvider) calculateCost(model string, inputTokens, outputTokens int) float64 {
    // Pricing per 1M tokens (as of 2024)
    prices := map[string]struct{ input, output float64 }{
        "claude-3-haiku-20240307":  {0.25, 1.25},
        "claude-3-sonnet-20240229": {3.0, 15.0},
        "claude-3-opus-20240229":   {15.0, 75.0},
        "claude-3-5-sonnet-20241022": {3.0, 15.0},
    }

    price, ok := prices[model]
    if !ok {
        price = prices["claude-3-haiku-20240307"] // Default to cheapest
    }

    return (float64(inputTokens) * price.input / 1_000_000) +
           (float64(outputTokens) * price.output / 1_000_000)
}
```

```go
// internal/llm/providers/ollama.go
package providers

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/daszybak/prediction_markets/internal/llm"
)

type OllamaProvider struct {
    baseURL    string
    httpClient *http.Client
}

type OllamaConfig struct {
    BaseURL string // Default: http://localhost:11434
    Timeout time.Duration
}

func NewOllama(cfg OllamaConfig) *OllamaProvider {
    baseURL := cfg.BaseURL
    if baseURL == "" {
        baseURL = "http://localhost:11434"
    }

    return &OllamaProvider{
        baseURL: baseURL,
        httpClient: &http.Client{
            Timeout: cfg.Timeout,
        },
    }
}

func (p *OllamaProvider) Name() string {
    return "ollama"
}

func (p *OllamaProvider) Complete(ctx context.Context, req *llm.Request) (*llm.Response, error) {
    start := time.Now()

    ollamaReq := map[string]any{
        "model":  req.Model,
        "stream": false,
        "messages": req.Messages,
    }

    if req.JSONMode {
        ollamaReq["format"] = "json"
    }

    if req.Temperature > 0 {
        ollamaReq["options"] = map[string]any{
            "temperature": req.Temperature,
        }
    }

    body, _ := json.Marshal(ollamaReq)

    httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/chat", bytes.NewReader(body))
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }
    httpReq.Header.Set("Content-Type", "application/json")

    resp, err := p.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("do request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("ollama error: status %d", resp.StatusCode)
    }

    var ollamaResp struct {
        Message struct {
            Content string `json:"content"`
        } `json:"message"`
        Model          string `json:"model"`
        PromptEvalCount int   `json:"prompt_eval_count"`
        EvalCount       int   `json:"eval_count"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
        return nil, fmt.Errorf("decode response: %w", err)
    }

    return &llm.Response{
        Content:      ollamaResp.Message.Content,
        Model:        ollamaResp.Model,
        InputTokens:  ollamaResp.PromptEvalCount,
        OutputTokens: ollamaResp.EvalCount,
        Latency:      time.Since(start),
        Cost:         0, // Local = free
    }, nil
}

func (p *OllamaProvider) EstimateCost(req *llm.Request) float64 {
    return 0 // Local inference is free
}
```

### Budget Management

```go
// internal/llm/budget.go
package llm

import (
    "context"
    "fmt"
    "log/slog"
    "sync"
    "time"
)

type Budget struct {
    mu          sync.Mutex
    dailyLimit  float64
    spentToday  float64
    lastReset   time.Time
    logger      *slog.Logger
}

type BudgetConfig struct {
    DailyLimitUSD float64
    Logger        *slog.Logger
}

func NewBudget(cfg BudgetConfig) *Budget {
    return &Budget{
        dailyLimit: cfg.DailyLimitUSD,
        lastReset:  time.Now().Truncate(24 * time.Hour),
        logger:     cfg.Logger,
    }
}

func (b *Budget) CanSpend(estimatedCost float64) bool {
    b.mu.Lock()
    defer b.mu.Unlock()

    b.maybeReset()

    if b.spentToday + estimatedCost > b.dailyLimit {
        b.logger.Warn("LLM budget exhausted",
            "spent", b.spentToday,
            "limit", b.dailyLimit,
            "requested", estimatedCost)
        return false
    }
    return true
}

func (b *Budget) Record(actualCost float64) {
    b.mu.Lock()
    defer b.mu.Unlock()

    b.maybeReset()
    b.spentToday += actualCost

    b.logger.Debug("LLM cost recorded",
        "cost", actualCost,
        "spent_today", b.spentToday,
        "remaining", b.dailyLimit - b.spentToday)
}

func (b *Budget) maybeReset() {
    today := time.Now().Truncate(24 * time.Hour)
    if today.After(b.lastReset) {
        b.logger.Info("LLM budget reset",
            "previous_spend", b.spentToday)
        b.spentToday = 0
        b.lastReset = today
    }
}

func (b *Budget) Status() (spent, remaining float64) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.maybeReset()
    return b.spentToday, b.dailyLimit - b.spentToday
}
```

### Client with Provider Selection

```go
// internal/llm/client.go
package llm

import (
    "context"
    "fmt"
    "log/slog"
)

type Client struct {
    providers map[string]Provider
    primary   string
    fallback  string
    budget    *Budget
    logger    *slog.Logger
}

type ClientConfig struct {
    Providers map[string]Provider
    Primary   string // Primary provider name
    Fallback  string // Fallback provider name (optional)
    Budget    *Budget
    Logger    *slog.Logger
}

func NewClient(cfg ClientConfig) *Client {
    return &Client{
        providers: cfg.Providers,
        primary:   cfg.Primary,
        fallback:  cfg.Fallback,
        budget:    cfg.Budget,
        logger:    cfg.Logger,
    }
}

func (c *Client) Complete(ctx context.Context, req *Request) (*Response, error) {
    provider, ok := c.providers[c.primary]
    if !ok {
        return nil, fmt.Errorf("primary provider %q not found", c.primary)
    }

    // Check budget
    if c.budget != nil {
        estimatedCost := provider.EstimateCost(req)
        if !c.budget.CanSpend(estimatedCost) {
            // Try fallback (might be free local model)
            if c.fallback != "" {
                if fallbackProvider, ok := c.providers[c.fallback]; ok {
                    c.logger.Info("Using fallback provider due to budget",
                        "primary", c.primary,
                        "fallback", c.fallback)
                    provider = fallbackProvider
                }
            } else {
                return nil, fmt.Errorf("LLM budget exceeded and no fallback configured")
            }
        }
    }

    resp, err := provider.Complete(ctx, req)
    if err != nil {
        // Try fallback on error
        if c.fallback != "" && c.fallback != provider.Name() {
            if fallbackProvider, ok := c.providers[c.fallback]; ok {
                c.logger.Warn("Primary provider failed, using fallback",
                    "primary", c.primary,
                    "fallback", c.fallback,
                    "error", err)
                return fallbackProvider.Complete(ctx, req)
            }
        }
        return nil, err
    }

    // Record cost
    if c.budget != nil {
        c.budget.Record(resp.Cost)
    }

    return resp, nil
}

// ParseMarketRules extracts structured data from market rules text
func (c *Client) ParseMarketRules(ctx context.Context, rules string, platform string) (*ParsedMarketRules, error) {
    // Implementation in prompts/market_parse.go
    return parseMarketRules(ctx, c, rules, platform)
}

// VerifyMarketMatch checks if two markets are equivalent
func (c *Client) VerifyMarketMatch(ctx context.Context, marketA, marketB MarketForMatching) (*MarketMatchResult, error) {
    // Implementation in prompts/market_match.go
    return verifyMarketMatch(ctx, c, marketA, marketB)
}

// AnalyzeNewsImpact determines how news affects a market
func (c *Client) AnalyzeNewsImpact(ctx context.Context, news string, market MarketForAnalysis) (*NewsImpactResult, error) {
    // Implementation in prompts/news_impact.go
    return analyzeNewsImpact(ctx, c, news, market)
}

// MarketForMatching contains market data for matching
type MarketForMatching struct {
    Platform    string
    Question    string
    Description string
    RulesText   string // Combined rules (Kalshi: primary+secondary)
    EndDate     string
}

// MarketForAnalysis contains market data for news analysis
type MarketForAnalysis struct {
    Question string
    YesPrice float64
}
```

### Prompts

```go
// internal/llm/prompts/market_match.go
package prompts

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/daszybak/prediction_markets/internal/llm"
)

func verifyMarketMatch(ctx context.Context, client *llm.Client, a, b llm.MarketForMatching) (*llm.MarketMatchResult, error) {
    prompt := fmt.Sprintf(`You are comparing two prediction market rules for equivalence.

Market A (%s):
Question: %s
Rules: %s
End Date: %s

Market B (%s):
Question: %s
Rules: %s
End Date: %s

Analyze whether these markets resolve to the SAME outcome under ALL scenarios.

Consider:
1. Subject matter (same event/metric?)
2. Threshold/condition (exact same criteria?)
3. Resolution timing (same deadline?)
4. Resolution source (same data source?)
5. Edge cases (what happens in ambiguous scenarios?)

Respond with JSON only:
{
    "is_equivalent": true/false,
    "confidence": 0.0-1.0,
    "reasoning": "brief explanation",
    "differences": ["list of differences if any"]
}`,
        a.Platform, a.Question, a.RulesText, a.EndDate,
        b.Platform, b.Question, b.RulesText, b.EndDate)

    resp, err := client.Complete(ctx, &llm.Request{
        Model: "claude-3-haiku-20240307", // or configured model
        Messages: []llm.Message{
            {Role: "user", Content: prompt},
        },
        Temperature: 0.1, // Low temp for consistency
        MaxTokens:   500,
        JSONMode:    true,
    })
    if err != nil {
        return nil, fmt.Errorf("llm complete: %w", err)
    }

    var result llm.MarketMatchResult
    if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
        return nil, fmt.Errorf("parse response: %w", err)
    }

    return &result, nil
}
```

```go
// internal/llm/prompts/market_parse.go
package prompts

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/daszybak/prediction_markets/internal/llm"
)

func parseMarketRules(ctx context.Context, client *llm.Client, rules string, platform string) (*llm.ParsedMarketRules, error) {
    prompt := fmt.Sprintf(`Extract structured data from this prediction market:

Platform: %s
Rules/Description:
%s

Extract the following fields and respond with JSON only:
{
    "subject": "main entity (Bitcoin, Trump, S&P 500, etc.)",
    "metric": "what is being measured (price, election outcome, etc.)",
    "condition": "comparison operator (>, <, =, >=, <=, between)",
    "threshold": "target value",
    "resolution_time": "ISO 8601 timestamp if mentioned, null otherwise",
    "resolution_source": "data source for resolution if mentioned",
    "outcomes": ["list of possible outcomes"],
    "canonical": "normalized single-sentence description for embedding"
}

The canonical field should be a clear, normalized statement like:
"Bitcoin price exceeds $100,000 USD by 2025-12-31"
"Donald Trump wins 2024 US Presidential Election"`,
        platform, rules)

    resp, err := client.Complete(ctx, &llm.Request{
        Model: "claude-3-haiku-20240307",
        Messages: []llm.Message{
            {Role: "user", Content: prompt},
        },
        Temperature: 0.1,
        MaxTokens:   500,
        JSONMode:    true,
    })
    if err != nil {
        return nil, fmt.Errorf("llm complete: %w", err)
    }

    var result llm.ParsedMarketRules
    if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
        return nil, fmt.Errorf("parse response: %w", err)
    }

    return &result, nil
}
```

---

## Configuration

```yaml
# configs/collector/config.yaml
llm:
  primary_provider: anthropic  # or "ollama" for local
  fallback_provider: ollama    # optional

  budget:
    daily_limit_usd: 10.0

  anthropic:
    api_key: ${ANTHROPIC_API_KEY}
    model: claude-3-haiku-20240307
    timeout: 30s

  openai:
    api_key: ${OPENAI_API_KEY}
    model: gpt-4o-mini
    timeout: 30s

  ollama:
    base_url: http://localhost:11434
    model: qwen2.5:3b
    timeout: 60s

matcher:
  embedding_model: BAAI/bge-small-en-v1.5
  similarity_threshold: 0.7
  max_candidates: 20
  verify_top_n: 5
```

```go
// internal/config/llm.go
package config

import "time"

type LLMConfig struct {
    PrimaryProvider  string          `yaml:"primary_provider"`
    FallbackProvider string          `yaml:"fallback_provider"`
    Budget           BudgetConfig    `yaml:"budget"`
    Anthropic        AnthropicConfig `yaml:"anthropic"`
    OpenAI           OpenAIConfig    `yaml:"openai"`
    Ollama           OllamaConfig    `yaml:"ollama"`
}

type BudgetConfig struct {
    DailyLimitUSD float64 `yaml:"daily_limit_usd"`
}

type AnthropicConfig struct {
    APIKey  string        `yaml:"api_key"`
    Model   string        `yaml:"model"`
    Timeout time.Duration `yaml:"timeout"`
}

type OpenAIConfig struct {
    APIKey  string        `yaml:"api_key"`
    Model   string        `yaml:"model"`
    Timeout time.Duration `yaml:"timeout"`
}

type OllamaConfig struct {
    BaseURL string        `yaml:"base_url"`
    Model   string        `yaml:"model"`
    Timeout time.Duration `yaml:"timeout"`
}

type MatcherConfig struct {
    EmbeddingModel      string  `yaml:"embedding_model"`
    SimilarityThreshold float64 `yaml:"similarity_threshold"`
    MaxCandidates       int     `yaml:"max_candidates"`
    VerifyTopN          int     `yaml:"verify_top_n"`
}
```

---

## Usage Example

```go
// cmd/collector/main.go (excerpt)
package main

import (
    "context"
    "log/slog"

    "github.com/daszybak/prediction_markets/internal/llm"
    "github.com/daszybak/prediction_markets/internal/llm/providers"
)

func setupLLMClient(cfg *config.LLMConfig, logger *slog.Logger) (*llm.Client, error) {
    providerMap := make(map[string]llm.Provider)

    // Setup Anthropic if configured
    if cfg.Anthropic.APIKey != "" {
        providerMap["anthropic"] = providers.NewAnthropic(providers.AnthropicConfig{
            APIKey:  cfg.Anthropic.APIKey,
            Timeout: cfg.Anthropic.Timeout,
        })
    }

    // Setup Ollama (local)
    providerMap["ollama"] = providers.NewOllama(providers.OllamaConfig{
        BaseURL: cfg.Ollama.BaseURL,
        Timeout: cfg.Ollama.Timeout,
    })

    budget := llm.NewBudget(llm.BudgetConfig{
        DailyLimitUSD: cfg.Budget.DailyLimitUSD,
        Logger:        logger,
    })

    return llm.NewClient(llm.ClientConfig{
        Providers: providerMap,
        Primary:   cfg.PrimaryProvider,
        Fallback:  cfg.FallbackProvider,
        Budget:    budget,
        Logger:    logger,
    }), nil
}

// Example usage in market matching
func matchNewMarket(ctx context.Context, client *llm.Client, store *store.Store, newMarket Market) error {
    // Stage 1: Parse rules
    parsed, err := client.ParseMarketRules(ctx, newMarket.Description, newMarket.Platform)
    if err != nil {
        return fmt.Errorf("parse rules: %w", err)
    }

    // Stage 2: Find candidates via embedding
    candidates, err := store.FindSimilarMarkets(ctx, parsed.Canonical, 20)
    if err != nil {
        return fmt.Errorf("find similar: %w", err)
    }

    // Stage 3: Field filtering
    filtered := filterByFields(candidates, parsed)

    // Stage 4: LLM verification (top 5 only)
    for _, candidate := range filtered[:min(5, len(filtered))] {
        result, err := client.VerifyMarketMatch(ctx,
            llm.MarketForMatching{
                Platform:    newMarket.Platform,
                Question:    newMarket.Question,
                Description: newMarket.Description,
                EndDate:     newMarket.EndDate,
            },
            llm.MarketForMatching{
                Platform:    candidate.Platform,
                Question:    candidate.Question,
                Description: candidate.Description,
                EndDate:     candidate.EndDate,
            })
        if err != nil {
            logger.Warn("LLM verification failed", "error", err)
            continue
        }

        // Store result
        if err := store.UpsertMarketPair(ctx, UpsertMarketPairParams{
            MarketIDA:    newMarket.ID,
            MarketIDB:    candidate.ID,
            IsEquivalent: result.IsEquivalent,
            Confidence:   result.Confidence,
            VerifiedBy:   "llm",
            LLMReasoning: result.Reasoning,
        }); err != nil {
            return fmt.Errorf("store pair: %w", err)
        }

        if result.IsEquivalent && result.Confidence > 0.8 {
            logger.Info("Found equivalent market",
                "market_a", newMarket.ID,
                "market_b", candidate.ID,
                "confidence", result.Confidence)
            break
        }
    }

    return nil
}
```

---

## Embedding Service (Python Sidecar Option)

If you prefer Python for embeddings (better library support), use a simple FastAPI sidecar:

```python
# embedding_service/main.py
from fastapi import FastAPI
from pydantic import BaseModel
from sentence_transformers import SentenceTransformer
import numpy as np

app = FastAPI()
model = SentenceTransformer('BAAI/bge-small-en-v1.5')

class EmbedRequest(BaseModel):
    texts: list[str]

class EmbedResponse(BaseModel):
    embeddings: list[list[float]]

@app.post("/embed", response_model=EmbedResponse)
def embed(req: EmbedRequest):
    embeddings = model.encode(req.texts, normalize_embeddings=True)
    return EmbedResponse(embeddings=embeddings.tolist())

@app.get("/health")
def health():
    return {"status": "ok", "model": model.get_sentence_embedding_dimension()}
```

```go
// internal/embedder/client.go
package embedder

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
)

type Client struct {
    baseURL    string
    httpClient *http.Client
}

func (c *Client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
    body, _ := json.Marshal(map[string][]string{"texts": texts})

    req, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/embed", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result struct {
        Embeddings [][]float32 `json:"embeddings"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    return result.Embeddings, nil
}
```

---

## Migration: Add Parsed Rules Column

```sql
-- db/migrations/000006_parsed_rules.up.sql
ALTER TABLE markets ADD COLUMN IF NOT EXISTS parsed_rules JSONB;

CREATE INDEX IF NOT EXISTS idx_markets_parsed_subject
ON markets ((parsed_rules->>'subject'));

CREATE INDEX IF NOT EXISTS idx_markets_parsed_resolution_time
ON markets ((parsed_rules->>'resolution_time'));

COMMENT ON COLUMN markets.parsed_rules IS 'LLM-extracted structured market rules';
```

```sql
-- db/migrations/000006_parsed_rules.down.sql
DROP INDEX IF EXISTS idx_markets_parsed_resolution_time;
DROP INDEX IF EXISTS idx_markets_parsed_subject;
ALTER TABLE markets DROP COLUMN IF EXISTS parsed_rules;
```

---

## Cost Projections

| Operation | Model | Est. Cost | Volume | Daily Cost |
|-----------|-------|-----------|--------|------------|
| Market parsing | Haiku | $0.001 | 50 new/day | $0.05 |
| Market matching | Haiku | $0.01 | 50×5 comparisons | $2.50 |
| News analysis | Haiku | $0.005 | 100 news/day | $0.50 |
| **Total** | | | | **~$3/day** |

With Ollama fallback for non-critical operations, actual API cost can be reduced to ~$1/day.
