package signalconsensus

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
)

// The C++ ConsensusParams test replays this fixture
// (src/calculation/tests/fixtures/consensus_params.json): the unsized playbook under
// the default config and under a non-default config, evaluated by Strategy.Evaluate
// on the same bars. Regenerate with
//
//	CONSENSUS_FIXTURE_WRITE=1 go test ./src/calculation-signal/playbooks/signal-consensus -run TestCppParamsFixture
//
// Without the variable the test fails if the checked-in file drifted from the Go output.

const paramsFixturePath = "../../../calculation/tests/fixtures/consensus_params.json"

type paramsCase struct {
	Name     string          `json:"name"`
	Config   json.RawMessage `json:"config"`
	Expected []fixtureFill   `json:"expected"`
}

type paramsDoc struct {
	InitialQuote float64      `json:"initial_quote"`
	Bars         []fixtureBar `json:"bars"`
	Cases        []paramsCase `json:"cases"`
}

// paramsRows: close, rsi, volume, atr. Signal 100, regime 95, macro 90, hist 1, mean 1.
func paramsBars() []fixtureBar {
	rows := [][4]float64{
		{99, 60, 1, 1}, {101, 60, 1, 1}, {102, 60, 1, 1}, // cross up: buy
		{99, 60, 1, 1}, {101, 60, 1, 1}, {101, 60, 1, 1}, // early cross down, cross up
		{99, 60, 1, 1}, {99, 46, 1, 1}, // cross down, weak rsi
		{101, 46, 1, 1}, {101, 60, 1, 1}, {101, 60, 1, 1}, // cross up on rsi 46
		{99, 60, 1, 1}, {99, 60, 1, 1}, {99, 60, 1, 1}, // cross down
		{99, 60, 0.6, 1}, {101, 60, 0.6, 1}, {101, 60, 1, 1}, // cross up on thin volume
		{101, 60, 1, 1}, {101, 60, 1, 1}, {101, 60, 1, 1}, {101, 60, 1, 1},
		{99, 60, 1, 1}, {99, 60, 1, 1}, {98.5, 60, 1, 1}, {96, 60, 1, 2}, {93, 60, 1, 2}, // stop zone
		{99, 60, 1, 1}, {101, 60, 1, 1}, {101, 60, 1.2, 1}, {101, 60, 1.4, 1}, {101, 60, 1, 1},
		{99, 60, 1, 1}, {99, 60, 1, 1},
	}
	out := make([]fixtureBar, len(rows))
	for i, r := range rows {
		out[i] = fixtureBar{OpenMS: int64(i) * 10_800_000, Close: r[0], Volume: r[2], Signal: 100,
			Regime: 95, Macro: 90, RSI: r[1], Hist: 1, ATR: r[3], VolMean: 1}
	}
	return out
}

func simulateUnsized(t *testing.T, cfgJSON json.RawMessage, bars []fixtureBar, quote float64) []fixtureFill {
	t.Helper()
	cash, base := quote, 0.0
	lastBuy, lastSell := -1, -1
	var out []fixtureFill
	for i, b := range bars {
		since := func(last int) int {
			if last < 0 {
				return 999
			}
			return i - last
		}
		ctx := framework.Context{
			Position: "flat", Equity: cash + base*b.Close, BarClose: b.Close, BarVolume: b.Volume,
			BarsSinceBuy: since(lastBuy), BarsSinceSell: since(lastSell),
			StrategyConfig: cfgJSON, Results: resultsOf(b),
		}
		if base > 0 {
			ctx.Position = "long"
		}
		if i > 0 {
			ctx.HasPreviousBar = true
			ctx.PreviousBarClose = bars[i-1].Close
			ctx.PreviousResults = resultsOf(bars[i-1])
		}
		dec, err := (Strategy{}).Evaluate(ctx)
		if err != nil {
			t.Fatal(err)
		}
		before := base
		switch dec.Decision {
		case "buy":
			base += cash / b.Close
			cash = 0
			lastBuy = i
		case "sell":
			cash += base * b.Close
			base = 0
			lastSell = i
		default:
			continue
		}
		out = append(out, fixtureFill{BarOpenMS: b.OpenMS, Side: dec.Decision,
			Reason: dec.RuleID, Qty: math.Abs(base - before), Price: b.Close})
	}
	return out
}

func generateParamsFixture(t *testing.T) paramsDoc {
	t.Helper()
	doc := paramsDoc{InitialQuote: 100000, Bars: paramsBars()}
	for _, c := range []struct{ name, cfg string }{
		{"default", `{}`},
		{"non_default", `{"rsiBull":56,"rsiBear":45,"volRatioBull":1.3,"volRatioBear":0.5,"atrStopMult":1,"minBarsFlatAfterSell":2,"minBarsLongBeforeSell":1}`},
	} {
		cfg := json.RawMessage(c.cfg)
		doc.Cases = append(doc.Cases, paramsCase{Name: c.name, Config: cfg,
			Expected: simulateUnsized(t, cfg, doc.Bars, doc.InitialQuote)})
	}
	return doc
}

func TestCppParamsFixture(t *testing.T) {
	doc := generateParamsFixture(t)
	if len(doc.Cases[0].Expected) == 0 || len(doc.Cases[1].Expected) == 0 ||
		bytes.Equal(mustJSON(t, doc.Cases[0].Expected), mustJSON(t, doc.Cases[1].Expected)) {
		t.Fatal("both cases must trade and differ")
	}
	got, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if os.Getenv("CONSENSUS_FIXTURE_WRITE") != "" {
		if err := os.WriteFile(paramsFixturePath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(filepath.Clean(paramsFixturePath))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, got) {
		t.Fatal("consensus_params.json differs from the Go output; regenerate with CONSENSUS_FIXTURE_WRITE=1")
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
