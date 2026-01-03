package price

import (
	"encoding/json"
	"testing"
)

func TestPriceUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Price
		wantErr bool
	}{
		{"zero", `"0"`, 0, false},
		{"one", `"1"`, 1_000_000, false},
		{"half", `"0.5"`, 500_000, false},
		{"quarter", `"0.25"`, 250_000, false},
		{"typical price", `"0.123456"`, 123_456, false},
		{"needs padding 1 digit", `"0.1"`, 100_000, false},
		{"needs padding 2 digits", `"0.12"`, 120_000, false},
		{"needs padding 3 digits", `"0.123"`, 123_000, false},
		{"needs truncation", `"0.1234567"`, 123_456, false},
		{"raw number no quotes", `0.25`, 250_000, false},
		{"whole with frac", `"1.5"`, 1_500_000, false},
		{"two whole", `"2.0"`, 2_000_000, false},
		{"small frac", `"0.000001"`, 1, false},
		{"max precision", `"0.999999"`, 999_999, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Price
			err := got.UnmarshalJSON([]byte(tt.input))

			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPriceUnmarshalJSON_ViaJsonUnmarshal(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Price
	}{
		{"quoted string", `"0.5"`, 500_000},
		{"raw number", `0.75`, 750_000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Price
			if err := json.Unmarshal([]byte(tt.input), &got); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPriceInStruct(t *testing.T) {
	type Order struct {
		Price Price `json:"price"`
	}

	input := `{"price": "0.75"}`
	var o Order
	if err := json.Unmarshal([]byte(input), &o); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if o.Price != 750_000 {
		t.Errorf("got %d, want 750000", o.Price)
	}
}

func BenchmarkPriceUnmarshalJSON(b *testing.B) {
	data := []byte(`"0.123456"`)
	var p Price

	for i := 0; i < b.N; i++ {
		_ = p.UnmarshalJSON(data)
	}
}
