// Package websocket to get events of market and user data from Polymarket.
package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	HandshakeTimeout    = 30 * time.Second
	DefaultCloseTimeout = 5 * time.Second
	DefaultWriteTimeout = 10 * time.Second
	PingInterval        = 50 * time.Second
)

type Client struct {
	conn     *websocket.Conn
	stopPing chan struct{}
}

type Auth struct {
	APIKey     string `json:"apiKey"`
	Secret     string `json:"secret"`
	Passphrase string `json:"passphrase"`
}

type MarketSubscription struct {
	Auth        *Auth    `json:"auth"`
	AssetsIDs   []string `json:"assets_ids"`
	Type        string   `json:"type"`
	InitialDump *bool    `json:"initial_dump"`
}

func New(ctx context.Context, url string, endpoint string) (*Client, error) {
	dialer := websocket.Dialer{
		HandshakeTimeout: HandshakeTimeout,
	}

	conn, resp, err := dialer.DialContext(ctx, url+endpoint, http.Header{})
	if err != nil {
		return nil, err
	}
	log.Printf("Connected successfully to Polymarket websocket endpoint: %s. Polymarket websocket responded: %v", endpoint, resp.Status)

	c := &Client{
		conn:     conn,
		stopPing: make(chan struct{}),
	}
	go c.pingLoop()

	return c, nil
}

func (c *Client) pingLoop() {
	ticker := time.NewTicker(PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopPing:
			return
		case <-ticker.C:
			deadline := time.Now().Add(DefaultWriteTimeout)
			if err := c.conn.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
				log.Printf("failed to send ping: %v", err)
				return
			}
		}
	}
}

func (c *Client) Close(ctx context.Context) error {
	close(c.stopPing)

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(DefaultCloseTimeout)
	}

	err := c.conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		deadline,
	)
	if err != nil {
		log.Printf("failed to send close message: %v", err)
	}

	return c.conn.Close()
}

func (c *Client) SubscribeMarket(ctx context.Context, tokenIDs []string, initialDump bool, _ *Auth) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(DefaultWriteTimeout)
	}
	c.conn.SetWriteDeadline(deadline)

	sub := MarketSubscription{
		AssetsIDs:   tokenIDs,
		Type:        "market",
		InitialDump: &initialDump,
	}
	return c.conn.WriteJSON(sub)
}

type result struct {
	RawMessage []byte
	Error      error
}

func (c *Client) ReadMessage(ctx context.Context) (*Message, error) {
	resultCh := make(chan result, 1)

	go func() {
		_, msg, err := c.conn.ReadMessage()
		resultCh <- result{
			RawMessage: msg,
			Error:      err,
		}
	}()

	select {
	case <-ctx.Done():
		if err := c.conn.SetReadDeadline(time.Now()); err != nil {
			log.Printf("failed to set read deadline: %v", err)
		}
		return nil, fmt.Errorf("reading message: %w", ctx.Err())
	case result := <-resultCh:
		if result.Error != nil {
			return nil, fmt.Errorf("couldn't read message: %w", result.Error)
		}
		msg, err := c.ParseMessage(result.RawMessage)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse message: %w", err)
		}
		return msg, nil
	}
}

type Message struct {
	EventType      string `json:"event_type"`
	Book           *Book
	PriceChange    *PriceChange
	BestBidAsk     *BestBidAsk
	TickSizeChange *TickSizeChange
	LastTradePrice *LastTradePrice
	NewMarket      *NewMarket
	MarketResolved *MarketResolved
}

type Book struct {
	AssetID   string         `json:"asset_id"`
	Market    string         `json:"market"`
	Timestamp string         `json:"timestamp"`
	Hash      string         `json:"hash"`
	Buys      []OrderSummary `json:"buys"`
	Sells     []OrderSummary `json:"sells"`
}

type OrderSummary struct {
	Price string `json:"price"`
	Size  string `json:"size"`
}

type PriceChange struct {
	AssetID string `json:"asset_id"`
	Price   string `json:"price"`
	Size    string `json:"size"`
	Side    string `json:"side"`
	Hash    string `json:"hash"`
	BestBid string `json:"best_ask"`
	BestAsk string `json:"best_bid"`
}

type TickSizeChange struct {
	AssetID     string `json:"asset_id"`
	Market      string `json:"market"`
	OldTickSize string `json:"old_tick_size"`
	NewTickSize string `json:"new_tick_size"`
	Side        string `json:"side"`
	Timestamp   string `json:"timestamp"`
}

type LastTradePrice struct {
	AssetID    string `json:"asset_id"`
	FeeRateBPS string `json:"fee_rate_bps"`
	Market     string `json:"market"`
	Price      string `json:"price"`
	Side       string `json:"side"`
	Size       string `json:"size"`
	Timestamp  string `json:"timestamp"`
}

type BestBidAsk struct {
	MarketConditionID string `json:"market"`
	AssetID           string `json:"asset_id"`
	BestBid           string `json:"best_bid"`
	BestAsk           string `json:"best_ask"`
	Spread            string `json:"spread"`
	Timestamp         string `json:"timestamp"`
}

type NewMarket struct {
	MarketID          string       `json:"id"`
	Question          string       `json:"question"`
	MarketConditionID string       `json:"market"`
	Slug              string       `json:"slug"`
	Description       string       `json:"description"`
	AssetsIDs         []string     `json:"assets_ids"`
	Outcomes          []string     `json:"outcomes"`
	EventMessage      EventMessage `json:"event_message"`
	Timestamp         string       `json:"timestamp"`
}

type EventMessage struct {
	ID          string `json:"id"`
	Ticker      string `json:"ticker"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type MarketResolved struct {
	MarketID          string       `json:"id"`
	Question          string       `json:"question"`
	MarketConditionID string       `json:"market"`
	Timestamp         string       `json:"timestamp"`
	EventMessage      EventMessage `json:"event_message"`
}

const (
	BookEvent           = "book"
	PriceChangeEvent    = "price_change"
	TickSizeChangeEvent = "tick_size_change"
	BestBidAskEvent     = "best_bid_ask"
	NewMarketEvent      = "new_market"
	MarketResolvedEvent = "market_resolved"
)

func (c *Client) ParseMessage(msg []byte) (*Message, error) {
	base := &Message{}
	err := json.Unmarshal(msg, base)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse base message: %w", err)
	}

	switch base.EventType {
	case BookEvent:
		book := &Book{}
		err = json.Unmarshal(msg, book)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse book event: %w", err)
		}

		return &Message{
			EventType: BookEvent,
			Book:      book,
		}, nil
	case PriceChangeEvent:
		pC := &PriceChange{}
		err = json.Unmarshal(msg, pC)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse price change event: %w", err)
		}

		return &Message{
			EventType:   PriceChangeEvent,
			PriceChange: pC,
		}, nil
	case TickSizeChangeEvent:
		tSC := &TickSizeChange{}
		err = json.Unmarshal(msg, tSC)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse tick size change event: %w", err)
		}

		return &Message{
			EventType:      PriceChangeEvent,
			TickSizeChange: tSC,
		}, nil
	case BestBidAskEvent:
		bbA := &BestBidAsk{}
		err = json.Unmarshal(msg, bbA)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse best bid ask event: %w", err)
		}

		return &Message{
			EventType:  PriceChangeEvent,
			BestBidAsk: bbA,
		}, nil
	case NewMarketEvent:
		nM := &NewMarket{}
		err = json.Unmarshal(msg, nM)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse new market event: %w", err)
		}

		return &Message{
			EventType: PriceChangeEvent,
			NewMarket: nM,
		}, nil
	case MarketResolvedEvent:
		mR := &MarketResolved{}
		err = json.Unmarshal(msg, mR)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse market resolved event: %w", err)
		}

		return &Message{
			EventType:      PriceChangeEvent,
			MarketResolved: mR,
		}, nil
	default:
		return nil, fmt.Errorf("couldn't find event type")
	}
}
