// Package orderbook tracks the bids and asks for a tokenID.
package orderbook

import (
	"fmt"
	"time"

	"github.com/google/btree"

	"github.com/daszybak/prediction_markets/internal/price"
)

// Level represents a price level in the order book.
type Level struct {
	Price     price.Price
	Size      price.Size
	UpdatedAt time.Time // When this level was last updated (event time from source)
}

// lessAsc compares levels by price ascending (for asks: lowest first).
func lessAsc(a, b Level) bool {
	return a.Price < b.Price
}

// lessDesc compares levels by price descending (for bids: highest first).
func lessDesc(a, b Level) bool {
	return a.Price > b.Price
}

// Orderbook maintains sorted bid and ask levels using btrees.
// Bids are sorted descending (highest price first).
// Asks are sorted ascending (lowest price first).
type Orderbook struct {
	bids *btree.BTreeG[Level]
	asks *btree.BTreeG[Level]
}

// New creates a new empty order book.
func New() *Orderbook {
	return &Orderbook{
		bids: btree.NewG(32, lessDesc), // degree 32, descending
		asks: btree.NewG(32, lessAsc),  // degree 32, ascending
	}
}

// Set sets an absolute size at a price level.
// If size <= 0, the level is removed.
// eventTime is the timestamp from the source API (use time.Now() if unavailable).
func (ob *Orderbook) Set(p price.Price, size price.Size, side string, eventTime time.Time) error {
	tree, err := ob.getTree(side)
	if err != nil {
		return err
	}

	if size <= 0 {
		tree.Delete(Level{Price: p})
		return nil
	}

	tree.ReplaceOrInsert(Level{Price: p, Size: size, UpdatedAt: eventTime})
	return nil
}

// Update applies a delta to a price level.
// If the resulting size <= 0, the level is removed.
// eventTime is the timestamp from the source API (use time.Now() if unavailable).
func (ob *Orderbook) Update(p price.Price, delta price.Size, side string, eventTime time.Time) error {
	tree, err := ob.getTree(side)
	if err != nil {
		return err
	}

	// Find existing level
	existing, found := tree.Get(Level{Price: p})
	newSize := delta
	if found {
		newSize = existing.Size + delta
	}

	if newSize <= 0 {
		tree.Delete(Level{Price: p})
		return nil
	}

	tree.ReplaceOrInsert(Level{Price: p, Size: newSize, UpdatedAt: eventTime})
	return nil
}

// GetTopN returns the top N price levels for a side.
// Bids: highest prices first. Asks: lowest prices first.
func (ob *Orderbook) GetTopN(side string, n int) ([]Level, error) {
	tree, err := ob.getTree(side)
	if err != nil {
		return nil, err
	}

	levels := make([]Level, 0, min(n, tree.Len()))
	tree.Ascend(func(lvl Level) bool {
		levels = append(levels, lvl)
		return len(levels) < n
	})

	return levels, nil
}

// Len returns the number of levels on a side.
func (ob *Orderbook) Len(side string) int {
	tree, _ := ob.getTree(side)
	if tree == nil {
		return 0
	}
	return tree.Len()
}

func (ob *Orderbook) getTree(side string) (*btree.BTreeG[Level], error) {
	switch side {
	case "bids":
		return ob.bids, nil
	case "asks":
		return ob.asks, nil
	default:
		return nil, fmt.Errorf("invalid side: %s", side)
	}
}
