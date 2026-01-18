// Package gamma consume Polymarket gamma endpoints.
package gamma

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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

// TokenIDs handles the double-encoded JSON array from the API.
type TokenIDs []string

func (t *TokenIDs) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	return json.Unmarshal([]byte(s), (*[]string)(t))
}

type Market struct {
	ID           string   `json:"id"`
	ConditionID  string   `json:"condition_id"`
	Question     string   `json:"question"`
	Slug         string   `json:"slug"`
	Outcomes     string   `json:"outcomes"`
	ClobTokenIDs TokenIDs `json:"clobTokenIds"`
	Liquidity    float64  `json:"liquidity,string"`
	Volume       float64  `json:"volume,string"`
	Volume24hr   float64  `json:"volume24hr,string"`
}

type Event struct {
	ID      string    `json:"id"`
	Markets []*Market `json:"markets"`
}

func (c *Client) GetMarkets(liquidityNumMin float64) ([]*Market, error) {
	endpoint := fmt.Sprintf("/markets?liquidity_num_min=%f", liquidityNumMin)
	return httpclient.GetResource[[]*Market](c.httpClient, c.baseURL, endpoint, []int{200})
}

func (c *Client) GetEventBySlug(slug string) (*Event, error) {
	return httpclient.GetResource[*Event](c.httpClient, c.baseURL, "/events/slug/"+slug, []int{200})
}
