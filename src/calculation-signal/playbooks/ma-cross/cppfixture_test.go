package macross

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"testing"

	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/sizing"
	"github.com/MaxBernstetter/dagobert/src/internal/kpi"
)

// TestWriteCppSizingFixture regenerates the recorded fixture the C++ sma-cross
// strategy is checked against (src/calculation/tests/sma_cross_sizing_test.cpp).
// Run it from the repository root. It only runs when CPP_SIZING_FIXTURE names the output file:
//
//	CPP_SIZING_FIXTURE=$PWD/src/calculation/tests/fixtures/sma_cross_sizing.json \
//	  go test ./src/calculation-signal/playbooks/ma-cross -run TestWriteCppSizingFixture -count=1
//
// It drives the sized ma-cross Strategy.Evaluate over a seeded random walk with
// a frictionless wallet and unit tracking that mirrors runner.trackUnits.
func TestWriteCppSizingFixture(t *testing.T) {
	out := os.Getenv("CPP_SIZING_FIXTURE")
	if out == "" {
		t.Skip("CPP_SIZING_FIXTURE not set")
	}
	const (
		barMS   = int64(10_800_000)
		startMS = int64(1_704_067_200_000)
		initial = 10_000.0
		fastBar = 2 // fastHr 6 at 3h bars
		slowBar = 4 // slowHr 12
		atrBar  = 8 // 24h
		count   = 120
	)
	cfgJSON := `{"fastHr":6,"slowHr":12,"sizing":{"unitRiskPct":0.5,"heatPct":1,"maxUnits":3,"stopAtrMult":2,"addAtrMult":0.5}}`
	var cfgDoc struct {
		Sizing sizing.Config `json:"sizing"`
	}
	if err := json.Unmarshal([]byte(cfgJSON), &cfgDoc); err != nil {
		t.Fatal(err)
	}

	type bar struct {
		OpenMS int64   `json:"openMs"`
		High   float64 `json:"high"`
		Low    float64 `json:"low"`
		Close  float64 `json:"close"`
	}
	type action struct {
		BarOpenMS int64    `json:"barOpenMs"`
		Side      string   `json:"side"`
		Reason    string   `json:"reason"`
		Stop      *float64 `json:"stop,omitempty"`
	}
	type fill struct {
		Side  string  `json:"side"`
		Qty   float64 `json:"qty"`
		Price float64 `json:"price"`
	}

	for seed := int64(1); seed < 500; seed++ {
		rng := rand.New(rand.NewSource(seed))
		bars := make([]bar, count)
		price := 100.0
		drift := 0.0
		for i := range bars {
			if i%40 == 0 {
				drift = (rng.Float64() - 0.45) * 0.012
			}
			price *= 1 + drift + rng.NormFloat64()*0.012
			spread := price * (0.004 + rng.Float64()*0.012)
			bars[i] = bar{OpenMS: startMS + int64(i)*barMS, High: price + spread, Low: price - spread, Close: price}
		}

		// Indicators exactly as the C++ SDK defines them.
		sma := func(i, n int) (float64, bool) {
			if i < n-1 {
				return 0, false
			}
			var s float64
			for j := i - n + 1; j <= i; j++ {
				s += bars[j].Close
			}
			return s / float64(n), true
		}
		atrAt := make([]float64, count)
		atrOK := make([]bool, count)
		var atr, trSum float64
		trN := 0
		for i := 1; i < count; i++ {
			tr := math.Max(bars[i].High-bars[i].Low, math.Max(math.Abs(bars[i].High-bars[i-1].Close), math.Abs(bars[i].Low-bars[i-1].Close)))
			if trN < atrBar {
				trSum += tr
				trN++
				if trN < atrBar {
					continue
				}
				atr = trSum / atrBar
			} else {
				atr = (atr*(atrBar-1) + tr) / atrBar
			}
			atrAt[i], atrOK[i] = atr, true
		}
		results := func(i int) []kpi.Result {
			var res []kpi.Result
			add := func(def map[string]any, v float64) {
				d, _ := json.Marshal(def)
				val, _ := json.Marshal(v)
				res = append(res, kpi.Result{Def: d, Value: val, Unit: "price"})
			}
			if v, ok := sma(i, fastBar); ok {
				add(map[string]any{"type": "MovingAverage", "method": "sma", "time_frame_hr": 6}, v)
			}
			if v, ok := sma(i, slowBar); ok {
				add(map[string]any{"type": "MovingAverage", "method": "sma", "time_frame_hr": 12}, v)
			}
			if atrOK[i] {
				add(map[string]any{"type": "Atr", "method": "wilder", "time_frame_hr": 24}, atrAt[i])
			}
			return res
		}

		cash, base := initial, 0.0
		var units []sizing.Unit
		var actions []action
		var fills []fill
		seen := map[string]bool{}
		for i, b := range bars {
			var lastFill float64
			if len(units) > 0 {
				lastFill = units[len(units)-1].Fill
			}
			position := "flat"
			if base > 0 {
				position = "long"
			}
			ctx := framework.Context{
				Equity: cash + base*b.Close, OpenUnits: append([]sizing.Unit(nil), units...), LastFill: lastFill,
				BarClose: b.Close, Position: position, Results: results(i),
				PreviousResults: nil, StrategyConfig: json.RawMessage(cfgJSON),
			}
			if i > 0 {
				ctx.HasPreviousBar = true
				ctx.PreviousResults = results(i - 1)
			}
			dec, err := Strategy{}.Evaluate(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if dec.Decision == "no-trade" {
				continue
			}
			before := base
			switch dec.Decision {
			case "buy":
				spend := math.Min(*dec.QuoteNotional, cash)
				cash -= spend
				base += spend / b.Close
				fills = append(fills, fill{"buy", spend / b.Close, b.Close})
				if dec.RaiseStopsBy != nil {
					for k := range units {
						units[k].Stop += math.Max(0, *dec.RaiseStopsBy)
					}
				}
				units = append(units, sizing.Unit{Fill: b.Close, Stop: *dec.StopPrice, Qty: base - before})
				actions = append(actions, action{b.OpenMS, "buy", dec.RuleID, dec.StopPrice})
			case "sell":
				sold := base
				if dec.BaseQty != nil {
					sold = math.Min(*dec.BaseQty, base)
				}
				cash += sold * b.Close
				if sold == base {
					base = 0
					units = nil
				} else {
					base -= sold
					for k := range units {
						units[k].Qty *= base / before
					}
				}
				fills = append(fills, fill{"sell", sold, b.Close})
				actions = append(actions, action{b.OpenMS, "sell", dec.RuleID, nil})
			}
			seen[dec.RuleID] = true
		}
		if !(seen["golden_cross"] && seen["atr_stop"] && seen["death_cross"] && seen["vol_cut"] && seen["pyramid"]) {
			continue
		}
		doc := map[string]any{
			"generatedBy": "go test ./src/calculation-signal/playbooks/ma-cross -run TestWriteCppSizingFixture (see cppfixture_test.go)",
			"seed":        seed, "barMs": barMS, "initialQuote": initial,
			"fastHr": 6, "slowHr": 12,
			"sizing":  json.RawMessage(`{"unitRiskPct":0.5,"heatPct":1,"maxUnits":3,"stopAtrMult":2,"addAtrMult":0.5}`),
			"bars":    bars,
			"actions": actions, "fills": fills,
			"finalCash": cash, "finalQty": base,
		}
		body, err := json.MarshalIndent(doc, "", " ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(out, append(body, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("seed %d: %d actions", seed, len(actions))
		return
	}
	t.Fatal("no seed covers every rule")
}
