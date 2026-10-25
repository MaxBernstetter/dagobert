package macross

import (
	"encoding/json"
	"testing"

	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
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
