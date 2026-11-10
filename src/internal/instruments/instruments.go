package instruments

import (
	"strings"

	"github.com/MaxBernstetter/dagobert/src/shared"
)

// Instrument is one traded series and the venue pair orders use.
// PriceSource is the candle provider. Venue is where orders go.
// Those are different markets today: Binance USDT klines, USDC spot pairs.
type Instrument struct {
	Series      string
	Venue       string
	VenueSymbol string
	Base        string
	Quote       string
	PriceSource string
	Position    bool
}

var all = []Instrument{
	{Series: "BTCUSDT", Venue: "bybit", VenueSymbol: "BTCUSDC", Base: "BTC", Quote: "USDC", PriceSource: shared.DataSourceBinance.String(), Position: true},
	{Series: "ETHUSDT", Venue: "bybit", VenueSymbol: "ETHUSDC", Base: "ETH", Quote: "USDC", PriceSource: shared.DataSourceBinance.String(), Position: true},
	{Series: "USDCUSDT", Venue: "", VenueSymbol: "", Base: "USDC", Quote: "USDT", PriceSource: shared.DataSourceBinance.String(), Position: false},
}

// BySeries returns a registered live/venue instrument for a lake symbol such as BTCUSDT.
func BySeries(series string) (Instrument, bool) {
	for _, item := range all {
		if item.Series == series {
			return item, true
		}
	}
	return Instrument{}, false
}

// Resolve returns a registered instrument, or a lake-only instrument synthesized
// from a pipeline symbol. Venue fields stay empty for synthesized rows — use for
// paper/backtest over any provider catalog pair; live venue orders still need BySeries.
func Resolve(series string) (Instrument, bool) {
	if inst, ok := BySeries(series); ok {
		return inst, true
	}
	sym := strings.ToUpper(strings.TrimSpace(series))
	base, quote, ok := shared.SplitPipelineSymbol(sym)
	if !ok {
		return Instrument{}, false
	}
	return Instrument{
		Series:      sym,
		VenueSymbol: sym, // backtest id; there is no live venue pair
		Base:        base,
		Quote:       quote,
		PriceSource: shared.DataSourceBinance.String(),
		Position:    true,
	}, true
}

// PriceProvider returns the candle lake provider id for a series.
// Defaults to binance when the series is unknown.
func PriceProvider(series string) string {
	if inst, ok := BySeries(series); ok && inst.PriceSource != "" {
		return inst.PriceSource
	}
	return shared.DataSourceBinance.String()
}

// WalletCoins are uppercase coins passed to GET /v5/account/wallet-balance.
func WalletCoins() []string {
	return []string{"USDC", "USDT", "BTC", "ETH"}
}

// Positions are the series the portfolio treats as holdings.
func Positions() []Instrument {
	var out []Instrument
	for _, item := range all {
		if item.Position {
			out = append(out, item)
		}
	}
	return out
}
