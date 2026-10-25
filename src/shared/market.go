package shared

import (
	"fmt"
	"strings"
)

// Market is a currency pair: price of one Base in Quote units.
type Market struct {
	Base  string
	Quote string
}

// NormalizeMarket uppercases and trims base/quote.
func NormalizeMarket(base, quote string) (Market, error) {
	m := Market{
		Base:  strings.ToUpper(strings.TrimSpace(base)),
		Quote: strings.ToUpper(strings.TrimSpace(quote)),
	}
	if err := ValidatePartitionValue(m.Base); err != nil {
		return Market{}, fmt.Errorf("base: %w", err)
	}
	if err := ValidatePartitionValue(m.Quote); err != nil {
		return Market{}, fmt.Errorf("quote: %w", err)
	}
	if m.Base == m.Quote {
		return Market{}, fmt.Errorf("base and quote must differ")
	}
	return m, nil
}

// PipelineSymbol is the series id used by KPI / signal / execution for a lake market.
// Binance (default) uses concatenated wire ids (BTCUSDT).
func PipelineSymbol(provider, base, quote string) string {
	m, err := NormalizeMarket(base, quote)
	if err != nil {
		return strings.ToUpper(strings.TrimSpace(base)) + strings.ToUpper(strings.TrimSpace(quote))
	}
	_ = provider // other providers may use a different encoding later
	return m.Base + m.Quote
}

// SplitPipelineSymbol splits a Binance-style concatenated id into base/quote.
func SplitPipelineSymbol(sym string) (base, quote string, ok bool) {
	sym = strings.ToUpper(strings.TrimSpace(sym))
	quotes := []string{"USDT", "USDC", "BUSD", "FDUSD", "TUSD", "BTC", "ETH", "BNB", "EUR", "TRY", "BRL", "GBP", "AUD", "USD"}
	for _, q := range quotes {
		if strings.HasSuffix(sym, q) && len(sym) > len(q) {
			return sym[:len(sym)-len(q)], q, true
		}
	}
	return "", "", false
}
