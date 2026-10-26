package runner

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"

	marketrunner "github.com/MaxBernstetter/dagobert/src/calculation-market-action/runner"
	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
	execrunner "github.com/MaxBernstetter/dagobert/src/execution/runner"
	"github.com/MaxBernstetter/dagobert/src/internal/venue/paper"
	"github.com/MaxBernstetter/dagobert/src/internal/venue/sim"
)

const (
	testAgg      = int64(3 * 60 * 60 * 1000)
	testPlaybook = "ma-cross"
	testSymbol   = "BTCUSDT"
)

func testBars(n int, seed float64) []StrategyKPIBar {
	bars := make([]StrategyKPIBar, n)
	for i := range bars {
		bars[i] = StrategyKPIBar{
			CloseMs: testAgg * int64(i+1),
			Close:   30000 + 2500*math.Sin(float64(i)/7+seed) + float64(i)*11,
		}
	}
	return bars
}

// testActions alternates buys and sells, including a buy the wallet cannot afford and an
// oversized sell, so rejection and clamping paths are replayed too.
func testActions(bars []StrategyKPIBar) []ReplayAction {
	var out []ReplayAction
	for i := 3; i < len(bars); i += 5 {
		a := ReplayAction{CandleCloseMs: bars[i].CloseMs, Decision: "buy", Size: 2500 + float64(i)}
		if i%3 == 0 {
			a.Decision, a.Size = "sell", 0.05+float64(i)/1000
		}
		if i == 8 {
			a.Decision, a.Size = "buy", 1e9
		}
		out = append(out, a)
	}
	return out
}

// goPathFills is the existing Go path: simulated execution flushed to SQLite, read back.
func goPathFills(t *testing.T, job KPIJob) []StrategyKPIFill {
	t.Helper()
	dir := t.TempDir()
	w := paper.Wallet{Coins: map[string]float64{}}
	for c, v := range job.Coins {
		w.Coins[c] = v
	}
	batch := execrunner.NewSimBatch()
	position := "flat"
	px := map[int64]float64{}
	for _, b := range job.Bars {
		px[b.CloseMs] = b.Close
	}
	for _, a := range job.Actions {
		size := a.Size
		step, err := execrunner.SimulateBarWithModel(&w, job.Playbook, job.Symbol, a.CandleCloseMs, a.Decision, &size, px[a.CandleCloseMs], position, job.Costs, true)
		if err != nil {
			t.Fatal(err)
		}
		batch.Add(step)
		position = step.Position
	}
	if err := batch.Flush(dir, &w); err != nil {
		t.Fatal(err)
	}
	fills, err := LoadInstanceFills(dir, job.Playbook)
	if err != nil {
		t.Fatal(err)
	}
	return fills
}

func relEq(a, b, tol float64) bool {
	if a == b {
		return true
	}
	return math.Abs(a-b) <= tol*math.Max(math.Abs(a), math.Abs(b))
}

func compareJSON(t *testing.T, path string, a, b any, tol float64) {
	t.Helper()
	switch av := a.(type) {
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok || len(av) != len(bv) {
			t.Errorf("%s: shape differs", path)
			return
		}
		for k, v := range av {
			compareJSON(t, path+"."+k, v, bv[k], tol)
		}
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			t.Errorf("%s: length differs", path)
			return
		}
		for i := range av {
			compareJSON(t, fmt.Sprintf("%s[%d]", path, i), av[i], bv[i], tol)
		}
	case float64:
		bv, ok := b.(float64)
		if !ok || !relEq(av, bv, tol) {
			t.Errorf("%s: %v != %v", path, a, b)
		}
	default:
		if a != b {
			t.Errorf("%s: %v != %v", path, a, b)
		}
	}
}

func testJob(key string, seed float64) KPIJob {
	bars := testBars(400, seed)
	return KPIJob{
		Key: key, Playbook: testPlaybook, Symbol: testSymbol,
		FromMs: bars[0].CloseMs - testAgg, ToMs: bars[len(bars)-1].CloseMs,
		StartCash: 10000,
		Coins:     map[string]float64{"USDC": 10000, "BTC": 0.05},
		Costs:     sim.Model{SpreadBps: 2, SlippageBps: 1, FeeBps: 10},
		Actions:   testActions(bars), Bars: bars,
	}
}

func TestReplayKPIMatchesGoPath(t *testing.T) {
	job := testJob("a", 0)
	want := goPathFills(t, job)
	got := ReplayKPI(job)
	if got.Err != nil {
		t.Fatal(got.Err)
	}
	if len(want) == 0 || len(got.Fills) != len(want) {
		t.Fatalf("fills: got %d want %d", len(got.Fills), len(want))
	}
	index := map[int64]StrategyKPIFill{}
	for _, f := range want {
		index[f.CandleCloseMs] = f
	}
	for _, f := range got.Fills {
		w, ok := index[f.CandleCloseMs]
		if !ok || w.Side != f.Side || !relEq(w.BaseQty, f.BaseQty, 1e-9) ||
			!relEq(w.QuoteQty, f.QuoteQty, 1e-9) || !relEq(w.Price, f.Price, 1e-9) {
			t.Errorf("fill %d: got %+v want %+v", f.CandleCloseMs, f, w)
		}
	}
	wantRep := framework.BuildInstanceReportFromSim(job.Run, job.Playbook, job.Symbol, job.FromMs, job.ToMs, job.StartCash, want, job.Bars)
	var a, b any
	ga, _ := json.Marshal(got.Report)
	wa, _ := json.Marshal(wantRep)
	_ = json.Unmarshal(ga, &a)
	_ = json.Unmarshal(wa, &b)
	compareJSON(t, "report", a, b, 1e-6)
}

func TestReplayKPIMissingBar(t *testing.T) {
	job := testJob("a", 0)
	job.Actions = []ReplayAction{{CandleCloseMs: 1, Decision: "buy", Size: 100}}
	if r := ReplayKPI(job); r.Err == nil {
		t.Fatal("expected error for action without bar")
	}
}

func TestRunKPIWorkersBatches(t *testing.T) {
	in := make(chan []KPIJob)
	out := make(chan []KPIResult)
	go RunKPIWorkers(4, in, out)
	go func() {
		for b := 0; b < 6; b++ {
			var batch []KPIJob
			for i := 0; i < 5; i++ {
				batch = append(batch, testJob(fmt.Sprintf("%d-%d", b, i), float64(b*5+i)))
			}
			in <- batch
		}
		close(in)
	}()
	seen := 0
	for batch := range out {
		for i, r := range batch {
			if r.Err != nil || len(r.Fills) == 0 || r.Key == "" {
				t.Errorf("batch item %d: %+v", i, r.Err)
			}
			seen++
		}
	}
	if seen != 30 {
		t.Fatalf("results %d, want 30", seen)
	}
}

func TestReplayKPIRescalesSizedActions(t *testing.T) {
	job := testJob("sized", 0)
	job.Costs = sim.Model{}
	job.Coins = map[string]float64{"USDC": 10000}
	job.Policy = marketrunner.Policy{TargetWeightPct: 100, ReserveQuote: 0, MinOrderQuote: 1}
	notional := 1000.0
	job.Actions = []ReplayAction{{
		CandleCloseMs: job.Bars[3].CloseMs, Decision: "buy",
		// Sized by a C++ book of 5000 equity; the Go wallet holds 10000, so the order doubles.
		Sizing: &ReplaySizing{QuoteNotional: &notional, Equity: 5000},
	}}
	res := ReplayKPI(job)
	if res.Err != nil || len(res.Fills) != 1 {
		t.Fatalf("%v %+v", res.Err, res.Fills)
	}
	if got := res.Fills[0].QuoteQty; math.Abs(got-2000) > 1e-6 {
		t.Fatalf("quote spend %v, want 2000", got)
	}

	job.Actions[0].Sizing.Equity = 0
	if res := ReplayKPI(job); res.Err == nil {
		t.Fatal("sized action without equity must fail the instance")
	}

	// No sized quantities: legacy sizing, buy up to the target weight of the policy.
	job.Actions[0].Sizing = &ReplaySizing{}
	res = ReplayKPI(job)
	if res.Err != nil || len(res.Fills) != 1 || math.Abs(res.Fills[0].QuoteQty-10000) > 1e-6 {
		t.Fatalf("legacy sizing: %v %+v", res.Err, res.Fills)
	}
}
