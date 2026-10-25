package signalconsensus

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/sizing"
	"github.com/MaxBernstetter/dagobert/src/internal/kpi"
)

// The C++ ConsensusSignal sizing test replays this fixture
// (src/calculation/tests/fixtures/consensus_sizing.json). The fixture is generated from
// the sized Go playbook: bars are the hand-written table below, expected rows are the
// fills of a minimal simulator that mirrors the backtest loop (buy spends the decision's
// QuoteNotional capped by cash, sell uses BaseQty or the whole base, units are tracked
// as in runner.trackUnits). Regenerate with
//
//	CONSENSUS_FIXTURE_WRITE=1 go test ./src/calculation-signal/playbooks/signal-consensus -run TestCppFixture
//
// Without the variable the test fails if the checked-in file drifted from the Go output.

const fixturePath = "../../../calculation/tests/fixtures/consensus_sizing.json"

type fixtureBar struct {
	OpenMS  int64   `json:"open_ms"`
	Close   float64 `json:"close"`
	Volume  float64 `json:"volume"`
	Signal  float64 `json:"signal"`
	Regime  float64 `json:"regime"`
	Macro   float64 `json:"macro"`
	RSI     float64 `json:"rsi"`
	Hist    float64 `json:"hist"`
	ATR     float64 `json:"atr"`
	VolMean float64 `json:"vol_mean"`
}

type fixtureFill struct {
	BarOpenMS int64   `json:"bar_open_ms"`
	Side      string  `json:"side"`
	Reason    string  `json:"reason"`
	Qty       float64 `json:"qty"`
	Price     float64 `json:"price"`
}

type fixtureDoc struct {
	InitialQuote float64         `json:"initial_quote"`
	Config       json.RawMessage `json:"config"`
	Bars         []fixtureBar    `json:"bars"`
	Expected     []fixtureFill   `json:"expected"`
}

func fixtureBars() []fixtureBar {
	// close, atr per bar; signal 100, regime 95, macro 90, rsi 60, hist 1, volume 2, mean 1.
	rows := [][2]float64{
		{98, 2}, {101, 2}, // entry: cross up
		{103, 2}, {105, 2}, {106, 2}, // pyramids, then maxUnits
		{106, 8}, {106, 8}, // ATR spike: volatility cut
		{99, 8}, {99, 2}, {99, 2}, // ATR stop
		{101, 2}, {99, 2}, {99, 2}, {99, 2}, {99, 2}, {99, 2}, {99, 2}, {99, 2}, // cross up inside the cooldown
		{98, 2}, {98, 2}, {101, 2}, {101, 2}, {101, 2}, {101, 2}, // re-entry
		{99, 2}, {99, 2}, // cross down: sma exit
	}
	out := make([]fixtureBar, len(rows))
	for i, r := range rows {
		out[i] = fixtureBar{OpenMS: int64(i) * 10_800_000, Close: r[0], Volume: 2, Signal: 100,
			Regime: 95, Macro: 90, RSI: 60, Hist: 1, ATR: r[1], VolMean: 1}
	}
	return out
}

func resultsOf(b fixtureBar) []kpi.Result {
	return kit(b.Signal, b.Regime, b.Macro, b.RSI, b.Hist, b.ATR, b.VolMean)
}

func generateFixture(t *testing.T) fixtureDoc {
	t.Helper()
	cfgJSON := json.RawMessage(`{"sizing":{"unitRiskPct":0.5,"heatPct":2,"maxUnits":3,"addAtrMult":0.5}}`)
	doc := fixtureDoc{InitialQuote: 100000, Config: cfgJSON, Bars: fixtureBars()}
	cash, base := doc.InitialQuote, 0.0
	var units []sizing.Unit
	lastBuy, lastSell := -1, -1
	for i, b := range doc.Bars {
		since := func(last int) int {
			if last < 0 {
				return 999
			}
			return i - last
		}
		ctx := framework.Context{
			Position:       "flat",
			Equity:         cash + base*b.Close,
			OpenUnits:      append([]sizing.Unit(nil), units...),
			BarClose:       b.Close,
			BarVolume:      b.Volume,
			BarsSinceBuy:   since(lastBuy),
			BarsSinceSell:  since(lastSell),
			StrategyConfig: cfgJSON,
		}
		if len(units) > 0 {
			ctx.Position = "long"
			ctx.LastFill = units[len(units)-1].Fill
		}
		ctx.Results = resultsOf(b)
		if i > 0 {
			p := doc.Bars[i-1]
			ctx.HasPreviousBar = true
			ctx.PreviousBarClose = p.Close
			ctx.PreviousResults = resultsOf(p)
		}
		dec, err := (Strategy{}).Evaluate(ctx)
		if err != nil {
			t.Fatal(err)
		}
		before := base
		switch dec.Decision {
		case "buy":
			spend := math.Min(*dec.QuoteNotional, cash)
			cash -= spend
			base += spend / b.Close
			if dec.RaiseStopsBy != nil {
				for j := range units {
					units[j].Stop += math.Max(0, *dec.RaiseStopsBy)
				}
			}
			units = append(units, sizing.Unit{Fill: b.Close, Stop: *dec.StopPrice, Qty: base - before})
			lastBuy = i
		case "sell":
			q := base
			if dec.BaseQty != nil {
				q = math.Min(*dec.BaseQty, base)
			}
			cash += q * b.Close
			base -= q
			if dec.BaseQty == nil || base <= 1e-12 {
				base, units = 0, nil
			} else {
				for j := range units {
					units[j].Qty *= base / before
				}
			}
			lastSell = i
		default:
			continue
		}
		doc.Expected = append(doc.Expected, fixtureFill{BarOpenMS: b.OpenMS, Side: dec.Decision,
			Reason: dec.RuleID, Qty: math.Abs(base - before), Price: b.Close})
	}
	return doc
}

func TestCppFixture(t *testing.T) {
	got, err := json.MarshalIndent(generateFixture(t), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if os.Getenv("CONSENSUS_FIXTURE_WRITE") != "" {
		if err := os.WriteFile(fixturePath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(filepath.Clean(fixturePath))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, got) {
		t.Fatal("consensus_sizing.json differs from the Go output; regenerate with CONSENSUS_FIXTURE_WRITE=1")
	}
}
