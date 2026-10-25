package signalconsensus

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/sizing"
	"github.com/MaxBernstetter/dagobert/src/internal/kpi"
)

func TestConsensusBuyWithRegimeAndMacd(t *testing.T) {
	s := Strategy{}
	ctx := framework.Context{
		Position:         "flat",
		HasPreviousBar:   true,
		PreviousBarClose: 99,
		BarClose:         110,
		BarVolume:        120,
		BarsSinceSell:    defaultMinBarsFlatAfterSell,
		Results:          kit(102, 100, 95, 55, 0.5, 2.0, 100),
		PreviousResults:  kit(100, 99, 94, 50, -0.1, 1.0, 100),
	}
	dec, err := s.Evaluate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Decision != "buy" || dec.RuleID != "consensus_buy" {
		t.Fatalf("got %+v", dec)
	}
}

func TestConsensusSellOnCrossBelow(t *testing.T) {
	s := Strategy{}
	ctx := framework.Context{
		Position:         "long",
		HasPreviousBar:   true,
		PreviousBarClose: 102,
		BarClose:         93,
		BarVolume:        50,
		BarsSinceBuy:     defaultMinBarsLongBeforeSell,
		Results:          kit(94, 98, 90, 45, -0.2, 1.0, 100),
		PreviousResults:  kit(99, 99, 88, 52, 0.1, 1.0, 100),
	}
	dec, err := s.Evaluate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Decision != "sell" || dec.RuleID != "consensus_sell" {
		t.Fatalf("got %+v", dec)
	}
}

func TestNoBuyWithoutHigherTrend(t *testing.T) {
	s := Strategy{}
	ctx := framework.Context{
		Position:         "flat",
		HasPreviousBar:   true,
		PreviousBarClose: 99,
		BarClose:         105,
		BarVolume:        120,
		BarsSinceSell:    defaultMinBarsFlatAfterSell,
		Results:          kit(102, 106, 95, 55, 0.5, 2.0, 100),
		PreviousResults:  kit(100, 105, 94, 50, -0.1, 1.0, 100),
	}
	dec, err := s.Evaluate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Decision != "no-trade" {
		t.Fatalf("expected below sma24, got %+v", dec)
	}
}

func TestNoBuyWhenMacroStackBearish(t *testing.T) {
	s := Strategy{}
	ctx := framework.Context{
		Position:         "flat",
		HasPreviousBar:   true,
		PreviousBarClose: 99,
		BarClose:         110,
		BarVolume:        120,
		BarsSinceSell:    defaultMinBarsFlatAfterSell,
		Results:          kit(102, 100, 105, 55, 0.5, 2.0, 100),
		PreviousResults:  kit(100, 99, 104, 50, -0.1, 1.0, 100),
	}
	dec, err := s.Evaluate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Decision != "no-trade" {
		t.Fatalf("expected sma24 < sma48 block, got %+v", dec)
	}
}

func TestBuyRespectsNonDefaultVolumeThresholds(t *testing.T) {
	s := Strategy{}
	cfg, err := json.Marshal(Config{
		RsiBull:               52,
		RsiBear:               48,
		VolRatioBull:          1.1,
		VolRatioBear:          0.95, // ratio 0.9 is bear → blocks buy
		AtrStopMult:           2.5,
		MinBarsFlatAfterSell:  8,
		MinBarsLongBeforeSell: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	base := framework.Context{
		Position:         "flat",
		HasPreviousBar:   true,
		PreviousBarClose: 99,
		BarClose:         110,
		BarVolume:        90,
		BarsSinceSell:    defaultMinBarsFlatAfterSell,
		Results:          kit(102, 100, 95, 55, 0.5, 2.0, 100),
		PreviousResults:  kit(100, 99, 94, 50, -0.1, 1.0, 100),
		StrategyConfig:   cfg,
	}
	dec, err := s.Evaluate(base)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Decision != "no-trade" {
		t.Fatalf("expected volume bear block with volRatioBear=0.95, got %+v", dec)
	}
	base.StrategyConfig = nil
	dec, err = s.Evaluate(base)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Decision != "buy" {
		t.Fatalf("expected buy with default volRatioBear=0.85, got %+v", dec)
	}
}

func kit(signal12, regime24, macro48, rsi, hist, atr, volMean float64) []kpi.Result {
	mk := func(typ, method string, hr int, v float64) kpi.Result {
		def := map[string]any{"type": typ, "time_frame_hr": hr}
		if method != "" {
			def["method"] = method
		}
		if typ == "Macd" {
			def = map[string]any{
				"type": "Macd", "method": "histogram",
				"fast_time_frame_hr": 12, "slow_time_frame_hr": 24, "signal_time_frame_hr": 9,
			}
		}
		raw, _ := json.Marshal(def)
		val, _ := json.Marshal(v)
		return kpi.Result{Def: raw, Value: val}
	}
	return []kpi.Result{
		mk("MovingAverage", "sma", signalTrendHr, signal12),
		mk("MovingAverage", "sma", regimeTrendHr, regime24),
		mk("MovingAverage", "sma", macroTrendHr, macro48),
		mk("Rsi", "", barHr, rsi),
		mk("Macd", "histogram", 3, hist),
		mk("Atr", "wilder", atrHr, atr),
		mk("VolumeMean", "", barHr, volMean),
	}
}

func sizedCfg(maxUnits int) json.RawMessage {
	return json.RawMessage(`{"sizing":{"maxUnits":` + string(rune('0'+maxUnits)) + `}}`)
}

func longCtx(close float64, units []sizing.Unit, equity float64, cfg json.RawMessage) framework.Context {
	return framework.Context{
		Position: "long", HasPreviousBar: true, PreviousBarClose: close, BarClose: close,
		BarsSinceBuy: defaultMinBarsLongBeforeSell, Equity: equity, OpenUnits: units,
		LastFill: units[len(units)-1].Fill, StrategyConfig: cfg,
		Results:         kit(100, 100, 95, 55, 0.5, 2.0, 100),
		PreviousResults: kit(100, 100, 95, 55, 0.5, 2.0, 100),
	}
}

func TestSizedBuyUsesFillBasedStop(t *testing.T) {
	ctx := framework.Context{
		Position: "flat", HasPreviousBar: true, PreviousBarClose: 99, BarClose: 110, BarVolume: 120,
		BarsSinceSell: defaultMinBarsFlatAfterSell, Equity: 100000, StrategyConfig: sizedCfg(1),
		Results:         kit(102, 100, 95, 55, 0.5, 2.0, 100),
		PreviousResults: kit(100, 99, 94, 50, -0.1, 1.0, 100),
	}
	dec, err := Strategy{}.Evaluate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// stop = 110 - 2.5*2 = 105; notional = 1000 / (5/110) = 22000.
	if dec.Decision != "buy" || dec.StopPrice == nil || *dec.StopPrice != 105 ||
		dec.QuoteNotional == nil || math.Abs(*dec.QuoteNotional-22000) > 1e-6 ||
		dec.RequestedRiskPct == nil || dec.CappedRiskPct == nil {
		t.Fatalf("got %+v", dec)
	}
}

func TestSizedExitsAndVolCut(t *testing.T) {
	units := []sizing.Unit{{Fill: 110, Stop: 105, Qty: 200}}
	cases := []struct {
		name    string
		ctx     framework.Context
		rule    string
		baseQty bool
	}{
		{"stop", longCtx(104, units, 100000, sizedCfg(1)), "atr_stop", false},
		{"vol cut", longCtx(112, units, 5000, sizedCfg(1)), "vol_cut", true},
	}
	for _, c := range cases {
		dec, err := Strategy{}.Evaluate(c.ctx)
		if err != nil {
			t.Fatal(err)
		}
		if dec.Decision != "sell" || dec.RuleID != c.rule || (dec.BaseQty != nil) != c.baseQty {
			t.Fatalf("%s: got %+v", c.name, dec)
		}
	}
	// 200*5 = 1000 risk vs 5000*2% = 100: sell (1000-100)/5 = 180.
	dec, _ := Strategy{}.Evaluate(cases[1].ctx)
	if math.Abs(*dec.BaseQty-180) > 1e-9 {
		t.Fatalf("vol cut qty %v", *dec.BaseQty)
	}
}

func TestSizedConsensusSellStillExits(t *testing.T) {
	units := []sizing.Unit{{Fill: 110, Stop: 80, Qty: 10}}
	ctx := longCtx(93, units, 100000, sizedCfg(1))
	ctx.PreviousBarClose = 102
	ctx.Results = kit(94, 98, 90, 45, -0.2, 1.0, 100)
	ctx.PreviousResults = kit(99, 99, 88, 52, 0.1, 1.0, 100)
	dec, err := Strategy{}.Evaluate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Decision != "sell" || dec.RuleID != "consensus_sell" || dec.BaseQty != nil {
		t.Fatalf("got %+v", dec)
	}
}

func TestPyramidOnlyWhenMaxUnitsAbove1(t *testing.T) {
	units := []sizing.Unit{{Fill: 110, Stop: 105, Qty: 200}}
	dec, _ := Strategy{}.Evaluate(longCtx(112, units, 100000, sizedCfg(2)))
	if dec.RuleID != "pyramid" || dec.Decision != "buy" || dec.RaiseStopsBy == nil || *dec.RaiseStopsBy != 1 {
		t.Fatalf("got %+v", dec)
	}
	dec, _ = Strategy{}.Evaluate(longCtx(112, units, 100000, sizedCfg(1)))
	if dec.Decision != "no-trade" {
		t.Fatalf("maxUnits 1 must not pyramid: %+v", dec)
	}
}

func TestInvalidSizingErrors(t *testing.T) {
	ctx := framework.Context{StrategyConfig: json.RawMessage(`{"sizing":{"unitRiskPct":5}}`)}
	if _, err := (Strategy{}).Evaluate(ctx); err == nil {
		t.Fatal("expected invalid sizing error")
	}
}
