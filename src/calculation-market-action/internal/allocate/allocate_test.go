package allocate

import (
	"math"
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

func fp(v float64) *float64 { return &v }

func TestNotionalBuyRejectsInvalidNotional(t *testing.T) {
	p := Policy{TargetWeightPct: 100, ReserveQuote: 0, MinOrderQuote: 1}
	for _, n := range []float64{0, -5, math.NaN(), math.Inf(1), math.Inf(-1)} {
		out := Size(Input{SignalDecision: "buy", QuoteCash: 10_000, MarkPrice: 80_000, QuoteNotional: fp(n)}, p)
		if out.Decision != "no-trade" || out.Note != "invalid_notional" || out.Size != nil {
			t.Errorf("notional %v: %+v", n, out)
		}
	}
}

func TestNotionalBuyCapsByCash(t *testing.T) {
	p := Policy{TargetWeightPct: 100, ReserveQuote: 50, MinOrderQuote: 1}
	out := Size(Input{SignalDecision: "buy", QuoteCash: 1_000, MarkPrice: 80_000, QuoteNotional: fp(5_000)}, p)
	if out.Decision != "buy" || out.Size == nil || *out.Size != 950 {
		t.Fatalf("%+v", out)
	}
}

func TestSellRejectsInvalidBaseQty(t *testing.T) {
	for _, q := range []float64{0, -0.1, math.NaN(), math.Inf(1)} {
		out := Size(Input{SignalDecision: "sell", BaseHeld: 0.5, MarkPrice: 80_000, BaseQty: fp(q)}, DefaultPolicy())
		if out.Decision != "no-trade" || out.Note != "invalid_base_qty" || out.Size != nil {
			t.Errorf("base qty %v: %+v", q, out)
		}
	}
}

func TestPartialSellBelowMinOrderIsSkipped(t *testing.T) {
	p := Policy{TargetWeightPct: 10, ReserveQuote: 0, MinOrderQuote: 100}
	out := Size(Input{SignalDecision: "sell", BaseHeld: 0.01, MarkPrice: 80_000, BaseQty: fp(0.0005)}, p)
	if out.Decision != "no-trade" || out.Note != "below_min_order" {
		t.Fatalf("%+v", out)
	}
	out = Size(Input{SignalDecision: "sell", BaseHeld: 0.01, MarkPrice: 80_000, BaseQty: fp(0.002)}, p)
	if out.Decision != "sell" || out.Size == nil || *out.Size != 0.002 {
		t.Fatalf("%+v", out)
	}
}
