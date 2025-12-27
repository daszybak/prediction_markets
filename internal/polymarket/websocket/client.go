// Package websocket to get events of market and user data from Polymarket.
package websocket

import (
	"context"
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
	APIKey    string `json:"apiKey"`
	Secret    string `json:"secret"`
	Passphras string `json:"passphrase"`
}

type MarketSubscription struct {
	Auth        *Auth    `json:"auth"`
	AssetsIDs   []string `json:"assets_ids"`
	Type        string   `json:"type"`
	InitialDump *bool    `json:"initial_dump"`
}

func New(ctx context.Context, url string) (*Client, error) {
	dialer := websocket.Dialer{
		HandshakeTimeout: HandshakeTimeout,
	}

	conn, resp, err := dialer.DialContext(ctx, url, http.Header{})
	if err != nil {
		return nil, err
	}
	log.Printf("Connected successfully to Polymarket websocket. Polymarket websocket responded: %v", resp.Status)

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

func (c *Client) ReadMessage(ctx context.Context) ([]byte, error) {
	done := make(chan struct{})
	var msg []byte
	var err error

	go func() {
		_, msg, err = c.conn.ReadMessage()
		close(done)
	}()

	select {
	case <-ctx.Done():
		if err = c.conn.SetReadDeadline(time.Now()); err != nil {
			log.Printf("failed to set read deadline: %v", err)
		}
		<-done
		return nil, fmt.Errorf("reading message: %w", ctx.Err())
	case <-done:
		log.Printf("message received: %s", msg)
		return msg, err
	}
}
