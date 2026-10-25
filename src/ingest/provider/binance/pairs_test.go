package binance

import "testing"

func TestParseExchangeInfoPairs(t *testing.T) {
	body := []byte(`{
		"symbols": [
			{"symbol":"BTCUSDT","status":"TRADING","baseAsset":"BTC","quoteAsset":"USDT","permissions":["SPOT"]},
			{"symbol":"ETHBTC","status":"TRADING","baseAsset":"ETH","quoteAsset":"BTC","permissions":["SPOT"]},
			{"symbol":"BTCBTC","status":"TRADING","baseAsset":"BTC","quoteAsset":"BTC","permissions":["SPOT"]},
			{"symbol":"XRPUSDT","status":"BREAK","baseAsset":"XRP","quoteAsset":"USDT","permissions":["SPOT"]},
			{"symbol":"ADAEUR","status":"TRADING","baseAsset":"ADA","quoteAsset":"EUR","permissions":["SPOT"]}
		]
	}`)
	pairs, err := parseExchangeInfoPairs(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 3 {
		t.Fatalf("len=%d want 3: %#v", len(pairs), pairs)
	}
	if pairs[0].Base != "ADA" || pairs[0].Quote != "EUR" {
		t.Fatalf("first=%+v", pairs[0])
	}
	if pairs[1].Base != "BTC" || pairs[1].Quote != "USDT" || pairs[1].NativeSymbol != "BTCUSDT" {
		t.Fatalf("btc=%+v", pairs[1])
	}
	if pairs[2].Base != "ETH" || pairs[2].Quote != "BTC" {
		t.Fatalf("eth=%+v", pairs[2])
	}
}

func TestNativeSymbol(t *testing.T) {
	if got := NativeSymbol("btc", "usdt"); got != "BTCUSDT" {
		t.Fatalf("%s", got)
	}
}
