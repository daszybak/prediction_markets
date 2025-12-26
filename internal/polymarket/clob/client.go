// Package clob is used to call clob polymarket endpoints.
package clob

import (
	"net/http"
	"time"

	"github.com/daszybak/prediction_markets/internal/polymarket/price"
	"github.com/daszybak/prediction_markets/pkg/httpclient"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
}

func New(baseURL string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
	}
}

type MarketToken struct {
	Outcome string                    `json:"outcome"`
	Price   polymarketprice.Price     `json:"price"`
	TokenID string                    `json:"token_id"`
	Winner  bool                      `json:"winner"`
}

type Market struct{
	ConditionID string `json:"condition_id"`
	Tokens MarketToken `json:"tokens"`
}

func (c *Client) GetMarketByConditionID(conditionID string) (*Market, error) {
	return httpclient.GetResource[*Market](c.httpClient, c.baseURL, "/markets"+conditionID, []int{200})
}
