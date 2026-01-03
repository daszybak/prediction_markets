// Package clob is used to call clob polymarket endpoints.
package clob

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/daszybak/prediction_markets/pkg/httpclient"
	"github.com/daszybak/prediction_markets/internal/price"
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
	Outcome string `json:"outcome"`
	Price   price.Price  `json:"price"`
	TokenID string `json:"token_id"`
	Winner  bool   `json:"winner"`
}

type Market struct {
	ConditionID string        `json:"condition_id"`
	Description string        `json:"description"`
	Question    string        `json:"question"`
	Tokens      []MarketToken `json:"tokens"`
}

type MarketPage struct {
	Limit      int       `json:"limit"`
	Count      int       `json:"count"`
	Data       []*Market `json:"data"`
	NextCursor *string   `json:"next_cursor,omitempty"`
}

func (c *Client) GetMarketByConditionID(conditionID string) (*Market, error) {
	market, err := httpclient.GetResource[*Market](c.httpClient, c.baseURL, "/markets/"+conditionID, []int{200})
	if err != nil {
		return nil, fmt.Errorf("couldn't get market by condition ID %s: %w", conditionID, err)
	}
	return market, nil
}

func (c *Client) GetMarkets(nextCursor *string) (*MarketPage, error) {
	endpoint := "/markets"
	if nextCursor != nil {
		endpoint += "?next_cursor=" + *nextCursor
	}
	markets, err := httpclient.GetResource[*MarketPage](c.httpClient, c.baseURL, endpoint, []int{200})
	if err != nil {
		return nil, fmt.Errorf("couldn't get markets by from next cursor: %w", err)
	}
	return markets, nil
}

func (c *Client) GetAllMarkets() ([]*Market, error) {
	markets := []*Market{}
	firstPage, err := c.GetMarkets(nil)
	if err != nil {
		return nil, fmt.Errorf("couldn't get first page of markets: %w", err)
	}
	markets = append(markets, firstPage.Data...)
	nextCursor := firstPage.NextCursor
	if nextCursor == nil {
		return markets, nil
	}
	for {
		page, err := c.GetMarkets(nextCursor)
		if err != nil {
			return nil, fmt.Errorf("couldn't get markets for next cursor %s: %w", *nextCursor, err)
		}
		markets = append(markets, page.Data...)
		if page.NextCursor != nil {
			nextCursor = page.NextCursor
			decodedNextCursor, _ := base64.StdEncoding.DecodeString(*page.NextCursor)
			if string(decodedNextCursor) == "-1" {
				break
			}
			continue
		}
		log.Println("received a market page")
		break
	}
	return markets, nil
}
