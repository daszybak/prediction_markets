// Package polymarket adapts Polymarket's APIs (CLOB, Gamma, WebSocket) to the Platform interface.
package polymarket

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/daszybak/prediction_markets/internal/polymarket/clob"
	"github.com/daszybak/prediction_markets/internal/polymarket/gamma"
	"github.com/daszybak/prediction_markets/internal/polymarket/websocket"
	"github.com/daszybak/prediction_markets/internal/store"
	"github.com/daszybak/prediction_markets/pkg/hashset"
	"github.com/jackc/pgx/v5/pgtype"
)

const platformName = "polymarket"

type Config struct {
	ClobURL            string
	GammaURL           string
	WebsocketURL       string
	MarketSyncInterval time.Duration
}

type Polymarket struct {
	config           Config
	store            *store.Store
	log              *slog.Logger
	subscribedTokens hashset.Set[string]

	clob  *clob.Client
	gamma *gamma.Client
	ws    *websocket.Client
}

// New creates a Polymarket client. Call Start() to connect.
func New(cfg Config, s *store.Store, log *slog.Logger) *Polymarket {
	return &Polymarket{
		config: cfg,
		store:  s,
		log:    log.With("component", platformName),
		clob:   clob.New(cfg.ClobURL),
		gamma:  gamma.New(cfg.GammaURL),
	}
}

// Start connects the websocket and begins reading messages.
// This method blocks until ctx is cancelled.
func (p *Polymarket) Start(ctx context.Context) error {
	p.log.Info("starting")

	// Connect websocket
	ws, err := websocket.New(ctx, p.config.WebsocketURL)
	if err != nil {
		return fmt.Errorf("websocket connect: %w", err)
	}
	p.ws = ws
	p.log.Info("websocket connected", "url", p.config.WebsocketURL)

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
			// TODO: Process message (update order book, record trade, etc.)
			p.log.Debug("message received", "size", len(msg))
		}
	}
}

// Stop closes the websocket connection.
func (p *Polymarket) Stop(ctx context.Context) error {
	if p.ws != nil {
		return p.ws.Close(ctx)
	}
	return nil
}

func (p *Polymarket) syncLoop(ctx context.Context) {
	// Sync markets before starting websocket
	if err := p.syncMarkets(ctx); err != nil {
		p.log.Error("initial market sync", "error", err)
	}
	tokenIDs, err := p.store.GetTokenIDsForPlatform(ctx, platformName)
	if err != nil {
		p.log.Error("intial market sync", "error", err)
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

			tokenIDs, err := p.store.GetTokenIDsForPlatform(ctx, platformName)
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
		var endDate pgtype.Timestamptz
		if m.EndDateISO != "" {
			t, err := time.Parse(time.RFC3339, m.EndDateISO)
			if err != nil {
				p.log.Warn("invalid end_date_iso", "market_id", m.ConditionID, "value", m.EndDateISO)
			} else {
				endDate = pgtype.Timestamptz{Time: t, Valid: true}
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

		// Upsert tokens.
		for _, t := range m.Tokens {
			if err := p.store.UpsertToken(ctx, store.UpsertTokenParams{
				ID:       t.TokenID,
				MarketID: m.ConditionID,
				Outcome:  t.Outcome,
			}); err != nil {
				return fmt.Errorf("upsert token %s: %w", t.TokenID, err)
			}
		}
	}

	// TODO Pair markets.

	p.log.Info("synced markets", "count", len(markets))
	return nil
}

func (p *Polymarket) subscribeToMarkets(ctx context.Context, tokenIDs []string) error {
	if len(tokenIDs) == 0 {
		p.log.Warn("no tokens to subscribe to")
		return nil
	}

	if err := p.ws.SubscribeMarket(ctx, tokenIDs, true, nil); err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}

	p.log.Info("subscribed to tokens", "count", len(tokenIDs))
	return nil
}
