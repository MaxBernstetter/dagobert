package backtest

import "testing"

func TestPaperVenueCoinsPutsSplitBookInVenueQuote(t *testing.T) {
	coins := paperVenueCoins("BTCUSDT", 50_000+50_000, 0)
	if coins["USDC"] != 100_000 {
		t.Fatalf("USDC = %v", coins["USDC"])
	}
	if coins["USDT"] != 0 {
		t.Fatalf("USDT = %v, want the whole book in USDC", coins["USDT"])
	}
	if coins["BTC"] != 0 {
		t.Fatalf("BTC = %v", coins["BTC"])
	}
}
