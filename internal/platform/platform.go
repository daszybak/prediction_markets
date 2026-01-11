// Package platform provides an adapter interface for prediction market platforms.
package platform

import (
	"context"
)

type Platform interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	// Health() HealthStatus
	// GetMarkets(ctx context.Context) ([]*store.Market, error)
	// SubscribeOrderBook(ctx context.Context, ids []string) (<-chan OrderBookUpdate, error)
}
