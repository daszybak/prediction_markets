// Package api is used to call Kalshi's API endpoints.
package api

import (
	"net/http"
	"time"
)

type Client struct {
	httpClient *http.Client
	APIKey     string
	baseURL    string
}

func New(baseURL string, apiKey string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
		APIKey:     apiKey,
	}
}

type Market struct {
	ConditionID string        `json:"condition_id"`
	Description string        `json:"description"`
	Question    string        `json:"question"`
	Tokens      []MarketToken `json:"tokens"`
}

type MarketPage struct {
	Markets []*Market `json:"markets"`
	Cursor  string   `json:"cursor,omitempty"`
}

func (c *Client) GetMarkets(nextCursor *string) (*MarketPage, error) {
}



