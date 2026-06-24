package money

import (
	"fmt"
	"math"
)

const CurrencyUSD = "USD"

type Amount struct {
	Currency  string `json:"currency"`
	MicroUSD  int64  `json:"amount_microusd"`
	AmountUSD string `json:"amount_usd,omitempty"`
}

func USDFromMicro(micro int64) Amount {
	return Amount{
		Currency:  CurrencyUSD,
		MicroUSD:  micro,
		AmountUSD: FormatUSD(micro),
	}
}

func MustUSDFromMicro(micro int64) (Amount, error) {
	if micro < 0 {
		return Amount{}, fmt.Errorf("amount must be non-negative")
	}
	return USDFromMicro(micro), nil
}

func AddMicro(a, b int64) (int64, error) {
	if a > math.MaxInt64-b {
		return 0, fmt.Errorf("amount overflow")
	}
	return a + b, nil
}

func SubMicro(a, b int64) (int64, error) {
	if b > a {
		return 0, fmt.Errorf("amount underflow")
	}
	return a - b, nil
}
