package macross

import (
	"encoding/json"
	"testing"

	"github.com/MaxBernstetter/trading-copilot/src/calculation-signal/internal/framework"
	"github.com/MaxBernstetter/trading-copilot/src/internal/kpi"
)

func TestGoldenCrossFromFlat(t *testing.T) {
	s := Strategy{}
	ctx := framework.Context{
		Position:        "flat",
		HasPreviousBar:  true,
		Results:         dualSMAResults(110, 100),
		PreviousResults: dualSMAResults(90, 100),
	}
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
	ctx := framework.Context{
		Position:        "long",
		HasPreviousBar:  true,
		Results:         dualSMAResults(90, 100),
		PreviousResults: dualSMAResults(110, 100),
	}
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
	ctx := framework.Context{
		Position:        "flat",
		HasPreviousBar:  true,
		Results:         dualSMAResults(110, 100),
		PreviousResults: dualSMAResults(108, 100),
	}
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
	ctx := framework.Context{
		Position:       "flat",
		HasPreviousBar: false,
		Results:        dualSMAResults(110, 100),
	}
	dec, err := s.Evaluate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Decision != "no-trade" {
		t.Fatalf("got %+v", dec)
	}
}

func dualSMAResults(fast, slow float64) []kpi.Result {
	mk := func(hr int, v float64) kpi.Result {
		def, _ := json.Marshal(map[string]any{
			"type": "MovingAverage", "method": "sma", "time_frame_hr": hr,
		})
		val, _ := json.Marshal(v)
		return kpi.Result{Def: def, Value: val, Unit: "price"}
	}
	return []kpi.Result{mk(3, fast), mk(6, slow)}
}
