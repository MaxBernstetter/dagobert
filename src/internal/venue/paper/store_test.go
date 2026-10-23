package paper

import (
	"testing"

	"github.com/MaxBernstetter/trading-copilot/src/internal/money"
)

func TestApplySellNeverExceedsBalance(t *testing.T) {
	dir := t.TempDir()
	if err := Reset(dir); err != nil {
		t.Fatal(err)
	}
	price := 80_000.0
	if err := ApplyBuy(dir, "USDC", "BTC", 800, price); err != nil {
		t.Fatal(err)
	}
	held, err := Coin(dir, "BTC")
	if err != nil {
		t.Fatal(err)
	}
	// Request slightly more than held (allocator float noise).
	if err := ApplySell(dir, "USDC", "BTC", money.FloorBase(held+1e-9), price); err != nil {
		t.Fatal(err)
	}
	after, err := Coin(dir, "BTC")
	if err != nil {
		t.Fatal(err)
	}
	if after != 0 {
		t.Fatalf("expected flat base, got %v", after)
	}
}
