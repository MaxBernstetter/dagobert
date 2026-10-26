package macross

import (
	"encoding/json"
	"testing"

	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/sizing"
	"github.com/MaxBernstetter/dagobert/src/internal/kpi"
)

func TestGoldenCrossFromFlat(t *testing.T) {
	s := Strategy{}
	ctx := frameworkContext(nil, "flat", true, dualSMAResults(110, 100), dualSMAResults(90, 100))
	dec, err := s.Evaluate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Decision != "buy" || dec.RuleID != "golden_cross" {
		t.Fatalf("got %+v", dec)
	}
}

func TestDeathCrossFromLong(t *testing.T) {
	s := Strategy{}
	ctx := frameworkContext(nil, "long", true, dualSMAResults(90, 100), dualSMAResults(110, 100))
	dec, err := s.Evaluate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Decision != "sell" || dec.RuleID != "death_cross" {
		t.Fatalf("got %+v", dec)
	}
}

func TestFastStillAboveNoBuy(t *testing.T) {
	s := Strategy{}
	ctx := frameworkContext(nil, "flat", true, dualSMAResults(110, 100), dualSMAResults(108, 100))
	dec, err := s.Evaluate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Decision != "no-trade" {
		t.Fatalf("expected no-trade when already above, got %+v", dec)
	}
}

func TestFirstBarNoTrade(t *testing.T) {
	s := Strategy{}
	ctx := frameworkContext(nil, "flat", false, dualSMAResults(110, 100), nil)
	dec, err := s.Evaluate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Decision != "no-trade" {
		t.Fatalf("got %+v", dec)
	}
}

func frameworkContext(cfg json.RawMessage, position string, hasPrev bool, results, prev []kpi.Result) framework.Context {
	return framework.Context{
		Position:        position,
		HasPreviousBar:  hasPrev,
		Results:         results,
		PreviousResults: prev,
		StrategyConfig:  cfg,
	}
}

func dualSMAResults(fast, slow float64) []kpi.Result {
	return dualSMAAt(3, 6, fast, slow)
}

func dualSMAAt(fastHr, slowHr int, fast, slow float64) []kpi.Result {
	mk := func(hr int, v float64) kpi.Result {
		def, _ := json.Marshal(map[string]any{
			"type": "MovingAverage", "method": "sma", "time_frame_hr": hr,
		})
		val, _ := json.Marshal(v)
		return kpi.Result{Def: def, Value: val, Unit: "price"}
	}
	return []kpi.Result{mk(fastHr, fast), mk(slowHr, slow)}
}

func sizedCtx(position string, bullish, bearish bool, atr, close, equity float64, units []sizing.Unit, extra ...kpi.Result) framework.Context {
	fast, slow, pf, ps := 100.0, 100.0, 100.0, 100.0
	switch {
	case bullish:
		pf, ps, fast, slow = 90, 100, 110, 100
	case bearish:
		pf, ps, fast, slow = 110, 100, 90, 100
	}
	atrDef, _ := json.Marshal(map[string]any{"type": "Atr", "method": "wilder", "time_frame_hr": 24})
	atrVal, _ := json.Marshal(atr)
	res := append(dualSMAResults(fast, slow), kpi.Result{Def: atrDef, Value: atrVal, Unit: "price"})
	ctx := frameworkContext(json.RawMessage(`{"sizing":{}}`), position, true, res, dualSMAResults(pf, ps))
	ctx.BarClose, ctx.Equity, ctx.OpenUnits = close, equity, units
	if len(units) > 0 {
		ctx.LastFill = units[len(units)-1].Fill
	}
	return ctx
}

func TestSizedGoldenCross(t *testing.T) {
	dec, err := Strategy{}.Evaluate(sizedCtx("flat", true, false, 2000, 100000, 100000, nil))
	if err != nil {
		t.Fatal(err)
	}
	if dec.RuleID != "golden_cross" || dec.QuoteNotional == nil || *dec.QuoteNotional != 25000 || *dec.StopPrice != 96000 {
		t.Fatalf("got %+v", dec)
	}
	if *dec.RequestedRiskPct != 1 || *dec.CappedRiskPct != 1 {
		t.Fatalf("risk %v %v", *dec.RequestedRiskPct, *dec.CappedRiskPct)
	}
}

func TestSizedBuyWithoutATRIsNoTrade(t *testing.T) {
	ctx := sizedCtx("flat", true, false, 2000, 100000, 100000, nil)
	ctx.Results = dualSMAResults(110, 100)
	dec, _ := Strategy{}.Evaluate(ctx)
	if dec.Decision != "no-trade" {
		t.Fatalf("got %+v", dec)
	}
}

func TestSizedLongOrder(t *testing.T) {
	unit := []sizing.Unit{{Fill: 100000, Stop: 96000, Qty: 0.25}}
	cases := []struct {
		name  string
		ctx   framework.Context
		rule  string
		dec   string
		check func(framework.Decision) bool
	}{
		{"stop beats death cross", sizedCtx("long", false, true, 2000, 95000, 100000, unit), "atr_stop", "sell", nil},
		{"death cross", sizedCtx("long", false, true, 2000, 105000, 100000, unit), "death_cross", "sell", nil},
		{"vol cut", sizedCtx("long", false, false, 5000, 105000, 100000, unit), "vol_cut", "sell", func(d framework.Decision) bool { return d.BaseQty != nil && *d.BaseQty > 0 }},
		{"hold", sizedCtx("long", false, false, 2000, 101000, 100000, unit), "none", "no-trade", nil},
		{"no pyramid at maxUnits 1", sizedCtx("long", false, false, 2000, 101500, 100000, unit), "none", "no-trade", nil},
	}
	for _, c := range cases {
		dec, err := Strategy{}.Evaluate(c.ctx)
		if err != nil {
			t.Fatal(c.name, err)
		}
		if dec.RuleID != c.rule || dec.Decision != c.dec || (c.check != nil && !c.check(dec)) {
			t.Errorf("%s: got %+v", c.name, dec)
		}
	}
}

func TestSizedPyramid(t *testing.T) {
	unit := []sizing.Unit{{Fill: 100000, Stop: 96000, Qty: 0.125}}
	ctx := sizedCtx("long", false, false, 2000, 101000, 100000, unit)
	ctx.StrategyConfig = json.RawMessage(`{"sizing":{"unitRiskPct":0.5,"maxUnits":2}}`)
	dec, err := Strategy{}.Evaluate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dec.RuleID != "pyramid" || dec.Decision != "buy" || *dec.RaiseStopsBy != 1000 || *dec.StopPrice != 97000 {
		t.Fatalf("got %+v", dec)
	}
	ctx.Results = sizedCtx("long", false, false, 2000, 100500, 100000, unit).Results
	ctx.BarClose = 100500
	if dec, _ := (Strategy{}).Evaluate(ctx); dec.Decision != "no-trade" {
		t.Fatalf("below 0.5 ATR: %+v", dec)
	}
}

func TestInvalidSizingRejected(t *testing.T) {
	ctx := sizedCtx("flat", true, false, 2000, 100000, 100000, nil)
	ctx.StrategyConfig = json.RawMessage(`{"sizing":{"unitRiskPct":3}}`)
	if _, err := (Strategy{}).Evaluate(ctx); err == nil {
		t.Fatal("expected error")
	}
}

func TestMinOrderQuoteSkipsSizedOrdersButNotExits(t *testing.T) {
	big := json.RawMessage(`{"sizing":{"unitRiskPct":0.5,"maxUnits":2,"minOrderQuote":1000000}}`)
	unit := []sizing.Unit{{Fill: 100000, Stop: 96000, Qty: 0.125}}
	with := func(ctx framework.Context) framework.Context { ctx.StrategyConfig = big; return ctx }
	for name, ctx := range map[string]framework.Context{
		"buy":     with(sizedCtx("flat", true, false, 2000, 100000, 100000, nil)),
		"vol cut": with(sizedCtx("long", false, false, 5000, 105000, 100000, unit)),
		"pyramid": with(sizedCtx("long", false, false, 2000, 101000, 100000, unit)),
	} {
		if dec, _ := (Strategy{}).Evaluate(ctx); dec.Decision != "no-trade" {
			t.Errorf("%s below minimum must be skipped: %+v", name, dec)
		}
	}
	for rule, ctx := range map[string]framework.Context{
		"atr_stop":    with(sizedCtx("long", false, true, 2000, 95000, 100000, unit)),
		"death_cross": with(sizedCtx("long", false, true, 2000, 105000, 100000, unit)),
	} {
		if dec, _ := (Strategy{}).Evaluate(ctx); dec.RuleID != rule {
			t.Errorf("full exit %s must run: %+v", rule, dec)
		}
	}
}
