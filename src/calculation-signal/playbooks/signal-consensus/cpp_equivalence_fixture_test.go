package signalconsensus

import (
	"bytes"
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/sizing"
	tirunner "github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/runner"
	"github.com/MaxBernstetter/dagobert/src/internal/kpi"
	"github.com/MaxBernstetter/dagobert/src/shared"
)

// The C++ ConsensusEquivalence test replays this fixture
// (src/calculation/tests/fixtures/consensus_equivalence.json). It holds a seeded series of
// 1h candles and, per config, the fills of the real Go path: candles -> technical-indicator
// job (3h aggregation, rsi-3h, volume-mean-3h, MACD, ATR, SMA) -> KPI window ->
// Strategy.Evaluate -> the minimal simulator below (the same loop as cpp_fixture_test.go).
// The C++ executable must reproduce the actions and fills from the candles alone.
// Regenerate with
//
//	CONSENSUS_FIXTURE_WRITE=1 go test ./src/calculation-signal/playbooks/signal-consensus -run TestCppEquivalenceFixture
//
// Without the variable the test fails if the checked-in file drifted from the Go output.

const equivalenceFixturePath = "../../../calculation/tests/fixtures/consensus_equivalence.json"

type eqCandle struct {
	OpenMS int64   `json:"open_ms"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

type eqCase struct {
	Name     string          `json:"name"`
	Config   json.RawMessage `json:"config"`
	Expected []fixtureFill   `json:"expected"`
}

type eqDoc struct {
	InitialQuote float64    `json:"initial_quote"`
	Candles      []eqCandle `json:"candles"`
	Cases        []eqCase   `json:"cases"`
}

const (
	eqStartMS = int64(1_704_067_200_000)
	eqHours   = 3 * 720
)

// eqCandles is a seeded hourly random walk with a slow trend cycle. A few hours are
// missing so that some 3h bars are incomplete and the playbook sees gaps.
func eqCandles() []eqCandle {
	rng := rand.New(rand.NewSource(128))
	missing := map[int]bool{401: true, 1002: true, 1003: true, 1500: true, 1801: true}
	round := func(v float64, places float64) float64 {
		p := math.Pow(10, places)
		return math.Round(v*p) / p
	}
	var out []eqCandle
	price := 1000.0
	for h := 0; h < eqHours; h++ {
		open := price
		drift := 0.0012 * math.Sin(2*math.Pi*float64(h)/420)
		price = round(price*math.Exp(drift+0.006*rng.NormFloat64()), 2)
		if price < 1 {
			price = 1
		}
		high := round(math.Max(open, price)+rng.Float64()*3, 2)
		low := round(math.Min(open, price)-rng.Float64()*3, 2)
		vol := round(5+10*rng.Float64(), 3)
		if missing[h] {
			continue
		}
		out = append(out, eqCandle{
			OpenMS: eqStartMS + int64(h)*hour, Open: open, High: high, Low: low, Close: price, Volume: vol,
		})
	}
	return out
}

func eqWriteLake(t *testing.T, dir string, candles []eqCandle) {
	t.Helper()
	if err := shared.EnsureDataLayout(dir); err != nil {
		t.Fatal(err)
	}
	rows := make([]hourCandle, len(candles))
	for i, c := range candles {
		rows[i] = hourCandle{OpenTime: c.OpenMS, Open: c.Open, High: c.High, Low: c.Low,
			Close: c.Close, Volume: c.Volume, CloseTime: c.OpenMS + hour - 1}
	}
	writeHourCandles(t, dir, rows)
}

// eqSimulate runs Strategy.Evaluate over the KPI window like FastSimulatedPipelineOpts:
// previous-bar results only when the prior 3h bar exists, cooldown counters in evaluated bars.
func eqSimulate(t *testing.T, cfgJSON json.RawMessage, win *kpi.Window, opens []int64, quote float64) []fixtureFill {
	t.Helper()
	aggMS := framework.AggregationDurationMS()
	cash, base := quote, 0.0
	var units []sizing.Unit
	lastBuy, lastSell := -1, -1
	var out []fixtureFill
	for i, open := range opens {
		snap, _ := win.Snapshot(open)
		since := func(last int) int {
			if last < 0 {
				return 999
			}
			return i - last
		}
		ctx := framework.Context{
			Position: "flat", Equity: cash + base*snap.Close,
			OpenUnits: append([]sizing.Unit(nil), units...),
			BarOpen:   open, BarClose: snap.Close, BarVolume: snap.Volume,
			BarsSinceBuy: since(lastBuy), BarsSinceSell: since(lastSell),
			StrategyConfig: cfgJSON, Results: snap.Results,
		}
		if base > 0 {
			ctx.Position = "long"
		}
		if len(units) > 0 {
			ctx.LastFill = units[len(units)-1].Fill
		}
		if prev, ok := win.Snapshot(open - aggMS); ok && open-aggMS >= 0 {
			ctx.HasPreviousBar = true
			ctx.PreviousBarClose = prev.Close
			ctx.PreviousResults = prev.Results
		}
		dec, err := (Strategy{}).Evaluate(ctx)
		if err != nil {
			t.Fatal(err)
		}
		before := base
		switch dec.Decision {
		case "buy":
			spend := cash
			if dec.QuoteNotional != nil {
				spend = math.Min(*dec.QuoteNotional, cash)
			}
			cash -= spend
			base += spend / snap.Close
			if dec.RaiseStopsBy != nil {
				for j := range units {
					units[j].Stop += math.Max(0, *dec.RaiseStopsBy)
				}
			}
			if dec.StopPrice != nil {
				units = append(units, sizing.Unit{Fill: snap.Close, Stop: *dec.StopPrice, Qty: base - before})
			}
			lastBuy = i
		case "sell":
			q := base
			if dec.BaseQty != nil {
				q = math.Min(*dec.BaseQty, base)
			}
			cash += q * snap.Close
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
		out = append(out, fixtureFill{BarOpenMS: open, Side: dec.Decision,
			Reason: dec.RuleID, Qty: math.Abs(base - before), Price: snap.Close})
	}
	return out
}

func generateEquivalenceFixture(t *testing.T) eqDoc {
	t.Helper()
	doc := eqDoc{InitialQuote: 100000, Candles: eqCandles()}
	dir := t.TempDir()
	eqWriteLake(t, dir, doc.Candles)
	end := eqStartMS + eqHours*hour
	rows, err := tirunner.BuildKPIRange(dir, "BTCUSDT", 0, end, end)
	if err != nil {
		t.Fatal(err)
	}
	bodies := make(map[int64]string, len(rows))
	opens := make([]int64, 0, len(rows))
	for _, r := range rows {
		bodies[r.BarOpen] = r.Body
		opens = append(opens, r.BarOpen)
	}
	sort.Slice(opens, func(i, j int) bool { return opens[i] < opens[j] })
	if len(opens) < 500 {
		t.Fatalf("need at least 500 3h bars, got %d", len(opens))
	}
	win, err := kpi.LoadWindowBodies(bodies)
	if err != nil {
		t.Fatal(err)
	}
	sized := `"sizing":{"unitRiskPct":0.5,"heatPct":1,"maxUnits":3,"addAtrMult":0.5}`
	for _, c := range []struct {
		name, cfg string
		trades    bool
	}{
		{"default", `{}`, true},
		{"non_default", `{"rsiBull":55,"rsiBear":46,"volRatioBull":0.9,"volRatioBear":0.5,"atrStopMult":1.5,"minBarsFlatAfterSell":3,"minBarsLongBeforeSell":2}`, true},
		{"sized_default", `{` + sized + `}`, true},
		{"sized_non_default", `{"atrStopMult":2,"minBarsFlatAfterSell":4,"rsiBull":51,"rsiBear":49,` + sized + `}`, true},
		// volume-mean-3h is the bar itself, so volume/mean is exactly 1: a bear ceiling of 1 vetoes every entry.
		{"volume_ratio_always_one", `{"volRatioBull":1.2,"volRatioBear":1}`, false},
	} {
		cfg := json.RawMessage(c.cfg)
		fills := eqSimulate(t, cfg, win, opens, doc.InitialQuote)
		if c.trades && len(fills) < 10 || !c.trades && len(fills) != 0 {
			t.Fatalf("case %s: %d fills", c.name, len(fills))
		}
		t.Logf("case %s: bars=%d fills=%d", c.name, len(opens), len(fills))
		doc.Cases = append(doc.Cases, eqCase{Name: c.name, Config: cfg, Expected: fills})
	}
	return doc
}

func TestCppEquivalenceFixture(t *testing.T) {
	got, err := json.MarshalIndent(generateEquivalenceFixture(t), "", " ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if os.Getenv("CONSENSUS_FIXTURE_WRITE") != "" {
		if err := os.WriteFile(equivalenceFixturePath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(filepath.Clean(equivalenceFixturePath))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, got) {
		t.Fatal("consensus_equivalence.json differs from the Go output; regenerate with CONSENSUS_FIXTURE_WRITE=1")
	}
}
