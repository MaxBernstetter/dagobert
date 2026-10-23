package money

import (
	"math"
	"testing"
)

func TestFloorBaseSellDoesNotExceedWallet(t *testing.T) {
	held := 0.073591539999999
	sized := FloorBase(held)
	if sized > held+1e-12 {
		t.Fatalf("sized %v > held %v", sized, held)
	}
	if math.Abs(sized-0.07359153) > 1e-8 {
		t.Fatalf("got %v", sized)
	}
}

func TestRoundQuote(t *testing.T) {
	if RoundQuote(4999.9999995) != 5000 {
		t.Fatalf("got %v", RoundQuote(4999.9999995))
	}
}
