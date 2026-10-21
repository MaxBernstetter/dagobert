package instruments

// Instrument is one traded series and the venue pair orders use.
// PriceSource is the candle provider. Venue is where orders go.
// Those are different markets today: Binance USDT klines, Bybit EU USDC spot.
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
	{Series: "BTCUSDT", Venue: "bybit", VenueSymbol: "BTCUSDC", Base: "BTC", Quote: "USDC", PriceSource: "binance", Position: true},
	{Series: "ETHUSDT", Venue: "bybit", VenueSymbol: "ETHUSDC", Base: "ETH", Quote: "USDC", PriceSource: "binance", Position: true},
	{Series: "USDCUSDT", Venue: "", VenueSymbol: "", Base: "USDC", Quote: "USDT", PriceSource: "binance", Position: false},
}

// BySeries returns the instrument for a lake symbol such as BTCUSDT.
func BySeries(series string) (Instrument, bool) {
	for _, item := range all {
		if item.Series == series {
			return item, true
		}
	}
	return Instrument{}, false
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
