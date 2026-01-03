// Package price handles price values from prediction market APIs
// without losing precision.
package price

import (
	"encoding/json"
)

type Price int64

var _ json.Unmarshaler = (*Price)(nil)

const PriceScale int64 = 1_000_000

func (p *Price) UnmarshalJSON(data []byte) error {
	if len(data) > 2 && data[0] == '"' && data[len(data)-1] == '"' {
		data = data[1 : len(data)-1]
	}
	// Else we assume that it is a raw number.

	var res int64
	i := 0

	for i < len(data) && data[i] != '.' {
		res = res*10 + int64(data[i]-'0')*PriceScale
		i++
	}

	if i < len(data) && data[i] == '.' {
		i++
		mult := PriceScale
		for i < len(data) {
			mult /= 10
			res += int64(data[i]-'0') * mult
			i++
		}
	}

	*p = Price(res)
	return nil
}
