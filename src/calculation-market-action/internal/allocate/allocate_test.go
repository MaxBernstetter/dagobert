package allocate

import (
	"testing"
)

func TestBuyRespectsTargetWeight(t *testing.T) {
	p := DefaultPolicy()
	in := Input{
		SignalDecision: "buy",
		QuoteCash:      10_000,
		BaseHeld:       0,
		MarkPrice:      80_000,
	}
	out := Size(in, p)
	if out.Decision != "buy" || out.Size == nil {
		t.Fatalf("%+v", out)
	}
	if *out.Size > 1000 { // 10% of 10k
		t.Fatalf("size %v", *out.Size)
	}
}

func TestBuyCanSpendEntireBook(t *testing.T) {
	out := Size(Input{
		SignalDecision: "buy",
		QuoteCash:      100_000,
		BaseHeld:       0,
		MarkPrice:      80_000,
	}, Policy{TargetWeightPct: 100, ReserveQuote: 0, MinOrderQuote: 1})
	if out.Decision != "buy" || out.Size == nil || *out.Size != 100_000 {
		t.Fatalf("%+v", out)
	}
}

func TestSellAllBase(t *testing.T) {
	out := Size(Input{
		SignalDecision: "sell",
		BaseHeld:       0.001,
		QuoteCash:      100,
		MarkPrice:      80_000,
	}, DefaultPolicy())
	if out.Decision != "sell" || out.Size == nil || *out.Size != 0.001 {
		t.Fatalf("%+v", out)
	}
}
