package sim

import (
	"math"
	"testing"
)

func TestBuyFillSpreadSlippageAndFee(t *testing.T) {
	m := Model{SpreadBps: 10, SlippageBps: 5, FeeBps: 10}
	mid := 50_000.0
	base, px, fee, gross, err := m.BuyFill(1000, mid)
	if err != nil {
		t.Fatal(err)
	}
	if gross != 1000 {
		t.Fatalf("gross %v", gross)
	}
	if fee <= 0 {
		t.Fatalf("fee %v", fee)
	}
	// half spread 5 bps + slippage 5 bps = 10 bps adverse
	wantPx := mid * 1.001
	if math.Abs(px-wantPx) > 0.02 {
		t.Fatalf("eff price %v want ~%v", px, wantPx)
	}
	if base <= 0 {
		t.Fatalf("base %v", base)
	}
}

func TestSellFillSpreadSlippageAndFee(t *testing.T) {
	m := Model{SpreadBps: 10, SlippageBps: 5, FeeBps: 10}
	mid := 50_000.0
	quote, px, fee, err := m.SellFill(0.01, mid)
	if err != nil {
		t.Fatal(err)
	}
	if fee <= 0 || quote <= 0 {
		t.Fatalf("quote %v fee %v", quote, fee)
	}
	wantPx := mid * 0.999
	if math.Abs(px-wantPx) > 0.02 {
		t.Fatalf("eff price %v want ~%v", px, wantPx)
	}
	gross := 0.01 * wantPx
	if quote >= gross {
		t.Fatalf("net quote should be below gross: %v vs %v", quote, gross)
	}
}
