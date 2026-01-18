// Package polymarket adapts Polymarket's APIs (CLOB, Gamma, WebSocket) to the Platform interface.
package polymarket

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/daszybak/prediction_markets/internal/engine"
	"github.com/daszybak/prediction_markets/internal/polymarket/clob"
	"github.com/daszybak/prediction_markets/internal/polymarket/gamma"
	"github.com/daszybak/prediction_markets/internal/polymarket/websocket"
	"github.com/daszybak/prediction_markets/internal/store"
	"github.com/daszybak/prediction_markets/pkg/hashset"
)

const platformName = "polymarket"

type Config struct {
	ClobURL            string
	GammaURL           string
	Websocket          Websocket
	MarketSyncInterval time.Duration
	MinLiquidity       float64
}

type Websocket struct {
	URL            string
	MarketEndpoint string
}

type Polymarket struct {
	config           Config
	store            *store.Store
	log              *slog.Logger
	subscribedTokens hashset.Set[string]
	subscribingMutex sync.Mutex
	engine           *engine.Client

	clob  *clob.Client
	gamma *gamma.Client
	ws    *websocket.Client
}

// New creates a Polymarket client. Call Start() to connect.
func New(cfg Config, s *store.Store, eng *engine.Client, log *slog.Logger) *Polymarket {
	return &Polymarket{
		config: cfg,
		store:  s,
		engine: eng,
		log:    log,
		clob:   clob.New(cfg.ClobURL),
		gamma:  gamma.New(cfg.GammaURL),
		subscribedTokens: hashset.NewSet[string](),
	}
}

// Start connects the websocket and begins reading messages.
// This method blocks until ctx is cancelled.
func (p *Polymarket) Start(ctx context.Context) error {
	p.log.Info("starting")

	// Connect websocket
	ws, err := websocket.New(ctx, p.config.Websocket.URL, p.config.Websocket.MarketEndpoint)
	if err != nil {
		return fmt.Errorf("websocket connect: %w", err)
	}
	p.ws = ws

	go p.syncLoop(ctx)

	// Read messages until context is cancelled
	for {
		select {
		case <-ctx.Done():
			p.log.Info("stopping", "reason", ctx.Err())
			return ctx.Err()
		default:
			msg, err := p.ws.ReadMessage(ctx)
			if err != nil {
				p.log.Error("read message failed", "error", err)
				return err
			}
			p.log.Debug("message received", "size", len(msg.EventType))
			p.processMessage(msg)
		}
	}
}

func (p *Polymarket) processMessage(msg *websocket.Message) {
	switch msg.EventType {
	case websocket.BookEvent:
		p.handleBook(msg.Book)
	case websocket.BookBatchEvent:
		for i := range msg.Books {
			p.handleBook(&msg.Books[i])
		}
		p.log.Info("processed initial book dump", "count", len(msg.Books))
	case websocket.PriceChangeEvent:
		p.handlePriceChange(msg.PriceChange)
	default:
		p.log.Debug("not handling message event type", "event_type", msg.EventType)
	}
}

func (p *Polymarket) handleBook(book *websocket.Book) {
	if book == nil {
		p.log.Warn("nil book in book event")
		return
	}

	p.log.Debug("handling book", "token", book.AssetID, "bids", len(book.Buys), "asks", len(book.Sells))

	// Parse event time from API.
	eventTime, err := time.Parse(time.RFC3339Nano, book.Timestamp)
	if err != nil {
		eventTime = time.Now()
	}

	// Process buys (bids).
	for _, order := range book.Buys {
		p.engine.Send(engine.Update{
			TokenID:   book.AssetID,
			Price:     order.Price,
			Size:      order.Size,
			Side:      "bids",
			EventTime: eventTime,
			IsDelta:   false, // Book is absolute snapshot
		})
	}

	// Process sells (asks).
	for _, order := range book.Sells {
		p.engine.Send(engine.Update{
			TokenID:   book.AssetID,
			Price:     order.Price,
			Size:      order.Size,
			Side:      "asks",
			EventTime: eventTime,
			IsDelta:   false,
		})
	}

	p.log.Debug("processed book", "token", book.AssetID, "bids", len(book.Buys), "asks", len(book.Sells))
}

func (p *Polymarket) handlePriceChange(pc *websocket.PriceChange) {
	if pc == nil {
		p.log.Warn("nil price_change in price_change event")
		return
	}

	side := "bids"
	if pc.Side == "sell" {
		side = "asks"
	}

	p.engine.Send(engine.Update{
		TokenID:   pc.AssetID,
		Price:     pc.Price,
		Size:      pc.Size,
		Side:      side,
		EventTime: time.Now(), // PriceChange doesn't have timestamp
		IsDelta:   false,      // Polymarket sends absolute sizes
	})
}

// Stop closes the websocket connection.
func (p *Polymarket) Stop(ctx context.Context) error {
	if p.ws != nil {
		return p.ws.Close(ctx)
	}
	return nil
}

func (p *Polymarket) syncLoop(ctx context.Context) {
	// Initial subscription using liquidity filter.
	tokenIDs, err := p.getLiquidTokenIDs()
	if err != nil {
		p.log.Error("initial market sync", "error", err)
	}

	if err := p.subscribeToMarkets(ctx, tokenIDs); err != nil {
		p.log.Error("initial market sync", "error", err)
	}

	ticker := time.NewTicker(p.config.MarketSyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := p.syncMarkets(ctx); err != nil {
				p.log.Error("syncing market", "error", err)
				continue
			}

			tokenIDs, err := p.getLiquidTokenIDs()
			if err != nil {
				p.log.Error("syncing market", "error", err)
				continue
			}

			if err := p.subscribeToMarkets(ctx, tokenIDs); err != nil {
				p.log.Error("syncing market", "error", err)
				continue
			}
		case <-ctx.Done():
			p.log.Info("market sync stopped", "reason", ctx.Err())
		}
	}
}

// syncMarkets fetches markets from the API and upserts them into the database.
func (p *Polymarket) syncMarkets(ctx context.Context) error {
	markets, err := p.clob.GetAllMarkets()
	if err != nil {
		return fmt.Errorf("get all markets: %w", err)
	}

	for _, m := range markets {
		// Parse end date.
		var endDate *time.Time
		if m.EndDateISO != "" {
			t, err := time.Parse(time.RFC3339, m.EndDateISO)
			if err != nil {
				p.log.Warn("invalid end_date_iso", "market_id", m.ConditionID, "value", m.EndDateISO)
			} else {
				endDate = &t
			}
		}

		// Upsert market.
		if err := p.store.UpsertMarket(ctx, store.UpsertMarketParams{
			ID:          m.ConditionID,
			Platform:    platformName,
			Description: m.Description,
			EndDate:     endDate,
		}); err != nil {
			return fmt.Errorf("upsert market %s: %w", m.ConditionID, err)
		}

		// Upsert tokens (ignore duplicate key errors for market_id/outcome constraint).
		for _, t := range m.Tokens {
			if err := p.store.UpsertToken(ctx, store.UpsertTokenParams{
				ID:       t.TokenID,
				MarketID: m.ConditionID,
				Outcome:  t.Outcome,
			}); err != nil {
				// Log and continue - happens when token ID changes for same market/outcome.
				p.log.Debug("upsert token skipped", "token", t.TokenID, "error", err)
			}
		}
	}

	// TODO Pair markets.

	p.log.Info("synced markets", "count", len(markets))
	return nil
}

// getLiquidTokenIDs returns token IDs for markets with liquidity >= minLiquidity.
func (p *Polymarket) getLiquidTokenIDs() ([]string, error) {
	markets, err := p.gamma.GetMarkets(p.config.MinLiquidity)
	if err != nil {
		return nil, fmt.Errorf("get gamma markets: %w", err)
	}

	var tokenIDs []string
	for _, m := range markets {
		tokenIDs = append(tokenIDs, m.ClobTokenIDs...)
	}

	p.log.Debug("fetched liquid markets", "markets", len(markets), "tokens", len(tokenIDs), "min_liquidity", p.config.MinLiquidity)
	return tokenIDs, nil
}

func (p *Polymarket) subscribeToMarkets(ctx context.Context, tokenIDs []string) error {
	p.subscribingMutex.Lock()
	defer p.subscribingMutex.Unlock()

	// Filter out already subscribed tokens.
	filtered := make([]string, 0, len(tokenIDs))
	for _, t := range tokenIDs {
		if !p.subscribedTokens.Has(t) {
			filtered = append(filtered, t)
		}
	}

	if len(filtered) == 0 {
		p.log.Info("no new tokens to subscribe to")
		return nil
	}

	if err := p.ws.SubscribeMarket(ctx, filtered, true, nil); err != nil {
		return fmt.Errorf("subscribe filtered %d: %w", len(filtered), err)
	}

	for _, t := range filtered {
		p.subscribedTokens.Set(t)
	}

	p.log.Info("subscribed to filtered tokens", "total", len(filtered))
	return nil
}
