// Package api is used to call Kalshi's API endpoints.
package api

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/daszybak/prediction_markets/pkg/httpclient"
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
	Ticker               string    `json:"ticker"`
	RulesPrimary         string    `json:"rules_primary"`
	RulesSecondary       string    `json:"rules_secondary"`
	LatestExpirationTime time.Time `json:"latest_expiration_time"`
}

type MarketPage struct {
	Markets []*Market `json:"markets"`
	Cursor  string    `json:"cursor"`
}

func (c *Client) GetMarkets(cursor string) (*MarketPage, error) {
	endpoint := "/markets"
	if cursor != "" {
		endpoint += "?cursor=" + cursor
	}
	markets, err := httpclient.GetResource[*MarketPage](c.httpClient, c.baseURL, endpoint, []int{200})
	if err != nil {
		return nil, fmt.Errorf("couldn't get markets by from cursor: %w", err)
	}
	return markets, nil
}

func (c *Client) GetAllMarkets() ([]*Market, error) {
	markets := []*Market{}
	firstPage, err := c.GetMarkets("")
	if err != nil {
		return nil, fmt.Errorf("couldn't get first page of markets: %w", err)
	}
	markets = append(markets, firstPage.Markets...)
	nextCursor := firstPage.Cursor
	for {
		page, err := c.GetMarkets(nextCursor)
		if err != nil {
			cursor := nextCursor
			if decoded, decodeErr := base64.StdEncoding.DecodeString(nextCursor); decodeErr == nil {
				cursor = string(decoded)
			}
			return markets, fmt.Errorf("couldn't get markets for cursor %s: %w", cursor, err)
		}
		markets = append(markets, page.Markets...)
		if page.Cursor != "" {
			nextCursor = page.Cursor
			decodedNextCursor, err := base64.StdEncoding.DecodeString(page.Cursor)
			if err != nil {
				return markets, fmt.Errorf("couldn't decode next cursor: %w", err)
			}
			if string(decodedNextCursor) == "-1" {
				break
			}
			continue
		}
		break
	}
	return markets, nil
}
