package money

import (
	"math"
	"strconv"
	"strings"
)

const (
	BaseDecimals  = 8
	QuoteDecimals = 6
)

// RoundBase normalizes a base-asset amount (BTC, ETH) for storage and comparison.
func RoundBase(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	p := math.Pow10(BaseDecimals)
	return math.Round(v*p) / p
}

// FloorBase rounds down so a sell size never exceeds the wallet (float noise).
func FloorBase(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return 0
	}
	p := math.Pow10(BaseDecimals)
	return math.Floor(v*p) / p
}

// RoundQuote normalizes a quote amount (USDC, USDT).
func RoundQuote(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	p := math.Pow10(QuoteDecimals)
	return math.Round(v*p) / p
}

// RoundCoin applies venue-style precision for a currency code.
func RoundCoin(currency string, v float64) float64 {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "BTC", "ETH":
		return RoundBase(v)
	default:
		return RoundQuote(v)
	}
}

// Parse parses a decimal string from the venue API into float64.
func Parse(s string) (float64, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, err
	}
	return f, nil
}
