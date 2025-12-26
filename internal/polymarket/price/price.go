// Package polymarketprice is used to handle price values
// coming from the polymarket API without losing precision.
package polymarketprice

import (
	"encoding/json"
)

type Price int64

var _ json.Unmarshaler = (*Price)(nil)

const PriceScale = 1_000_000

func (p *Price) UnmarshalJSON(data []byte) error {
	if len(data) > 2 && data[0] == '"' && data[len(data)-1] == '"' {
		data = data[1 : len(data)-1]
	}
	// Else we assume that it is a raw number.

	var res int64
	i := 0

	for i < len(data) {
		if data[i] == '.' {
			i++
			continue
		}

		res = res*10 + int64(data[i]-'0')*PriceScale
		i++
	}

	*p = Price(res)
	return nil
}
