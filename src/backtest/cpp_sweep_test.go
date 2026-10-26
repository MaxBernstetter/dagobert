package backtest

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	marketrunner "github.com/MaxBernstetter/dagobert/src/calculation-market-action/runner"
	signalrunner "github.com/MaxBernstetter/dagobert/src/calculation-signal/runner"
	tirunner "github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/runner"
	pb "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"github.com/MaxBernstetter/dagobert/src/internal/kpi"
	"github.com/MaxBernstetter/dagobert/src/internal/signalstore"
	"github.com/MaxBernstetter/dagobert/src/internal/venue/paper"
	"github.com/MaxBernstetter/dagobert/src/internal/venue/sim"
)

func TestReplayActionsSizedNeedsEquity(t *testing.T) {
	n := 100.0
	rep := &pb.RunReport{Key: "k", Actions: []*pb.ExperimentInstanceAction{
		{BarOpenMs: 0, Side: "buy", QuoteNotional: &n},
	}}
	if _, err := ReplayActions(rep, 10); err == nil {
		t.Fatal("sized action without equity must fail")
	}
	eq := 1000.0
	rep.Actions[0].Equity = &eq
	got, err := ReplayActions(rep, 10)
	if err != nil || len(got) != 1 || got[0].CandleCloseMs != 10 || got[0].Sizing.Equity != 1000 {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestSweepPayloadRejectsOtherPlaybooks(t *testing.T) {
	if _, err := SweepPayload("position-sizing", nil); err == nil {
		t.Fatal("unsupported playbook must fail loudly")
	}
}

func TestSweepPayloadSignalConsensus(t *testing.T) {
	a, err := SweepPayload("signal-consensus", []byte(`{"rsiBull":55,"minBarsFlatAfterSell":3,"sizing":{"maxUnits":2}}`))
	if err != nil {
		t.Fatal(err)
	}
	var m pb.SignalConsensusConfig
	if err := a.UnmarshalTo(&m); err != nil {
		t.Fatal(err)
	}
	if m.RsiBull != 55 || m.RsiBear != 48 || m.AtrStopMult != 2.5 || m.MinBarsFlatAfterSell != 3 || m.MinBarsLongBeforeSell != 4 {
		t.Fatalf("resolved config not sent in full: %+v", &m)
	}
	if m.Sizing == nil || m.Sizing.MaxUnits == nil || *m.Sizing.MaxUnits != 2 || m.Sizing.UnitRiskPct == nil || *m.Sizing.UnitRiskPct != 1 {
		t.Fatalf("sizing: %+v", m.Sizing)
	}
	if _, err := SweepPayload("signal-consensus", nil); err != nil {
		t.Fatalf("defaults: %v", err)
	}
}

func TestSweepPayloadSignalConsensusRejectsZeroCooldown(t *testing.T) {
	for _, c := range []string{`{"minBarsFlatAfterSell":0}`, `{"minBarsLongBeforeSell":0}`} {
		if _, err := SweepPayload("signal-consensus", []byte(c)); err == nil {
			t.Fatalf("%s must be rejected (#128)", c)
		}
	}
	if _, err := SweepPayload("signal-consensus", []byte(`{"rsiBull":40}`)); err == nil {
		t.Fatal("invalid config must fail Validate")
	}
}

func cppBinary(t *testing.T) string {
	t.Helper()
	return cppBinaryFor(t, "CPP_SMA_CROSS_BIN", "sma-cross", "sma-cross")
}

// cppBinaryFor resolves strategies/<dir>/strategy-<name> from env or the build presets, else skips.
func cppBinaryFor(t *testing.T, env, dir, name string) string {
	t.Helper()
	if p := os.Getenv(env); p != "" {
		return p
	}
	for _, preset := range []string{"optimized", "debug"} {
		p, _ := filepath.Abs(filepath.Join("..", "..", "build", preset, "strategies", dir, "strategy-"+name))
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skipf("strategy-%s not built (task cpp:build) and %s unset", name, env)
	return ""
}

// relEq is the equivalence tolerance of #130: relative with a tiny absolute floor.
func relEq(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol*math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
}

func compareJSON(t *testing.T, path string, a, b any, tol float64) {
	t.Helper()
	switch av := a.(type) {
	case map[string]any:
		bv, _ := b.(map[string]any)
		if len(av) != len(bv) {
			t.Errorf("%s: keys %d vs %d", path, len(av), len(bv))
		}
		for k, v := range av {
			compareJSON(t, path+"."+k, v, bv[k], tol)
		}
	case []any:
		bv, _ := b.([]any)
		if len(av) != len(bv) {
			t.Errorf("%s: len %d vs %d", path, len(av), len(bv))
			return
		}
		for i := range av {
			compareJSON(t, path+"[]", av[i], bv[i], tol)
		}
	case float64:
		bv, _ := b.(float64)
		if !relEq(av, bv, tol) {
			t.Errorf("%s: %v vs %v", path, av, bv)
		}
	default:
		if a != b {
			t.Errorf("%s: %v vs %v", path, a, b)
		}
	}
}

// TestSmaCrossGoVsCppEquivalence runs the same ma-cross configs on the Go experiment path and on
// the C++ sweep -> actions -> ReplayKPI path over one seeded lake. Fills must match in
// timestamp/side (exact) and qty/price (1e-9), instance KPIs within 1e-6.
// Sized configs are covered too: the C++ ATR is the simple mean of the last n true ranges, like Go.
//
// vol_cut and pyramid are covered: the replay recomputes the cut from the replayed wallet
// (heat_base_per_equity) and rescales pyramid notionals, see signalrunner.planReplay.
func TestSmaCrossGoVsCppEquivalence(t *testing.T) {
	bin := cppBinary(t)
	configs := []string{
		`{"fastHr":12,"slowHr":48}`,
		`{"fastHr":3,"slowHr":6}`,
		`{"fastHr":12,"slowHr":48,"sizing":{"heatPct":2,"stopAtrMult":0.5,"maxUnits":1}}`,
		`{"fastHr":6,"slowHr":12,"sizing":{"unitRiskPct":1,"heatPct":2,"stopAtrMult":0.5,"maxUnits":1}}`,
		`{"fastHr":12,"slowHr":48,"sizing":{}}`,
		`{"fastHr":6,"slowHr":12,"sizing":{"unitRiskPct":0.5,"maxUnits":3}}`,
		`{"fastHr":6,"slowHr":12,"sizing":{"unitRiskPct":0.5,"maxUnits":2}}`,
		`{"fastHr":3,"slowHr":12,"sizing":{"unitRiskPct":0.5,"heatPct":1,"maxUnits":4,"addAtrMult":0.25}}`,
		`{"fastHr":12,"slowHr":24,"sizing":{"unitRiskPct":0.75,"heatPct":1.5,"maxUnits":3,"stopAtrMult":1}}`,
	}
	rules := map[string]int{}
	for _, c := range goVsCppEquivalence(t, bin, "ma-cross", []string{"-seed", "7"}, configs) {
		for k, v := range c {
			rules[k] += v
		}
	}
	if rules["vol_cut"] == 0 || rules["pyramid"] == 0 {
		t.Fatalf("configs must fire vol_cut and pyramid, fired %v", rules)
	}
	t.Logf("rules fired: %v", rules)
}

// goVsCppEquivalence runs every config on the Go experiment path and on the C++ sweep -> actions
// -> ReplayKPI path over one seeded lake (genlakefixture -bench with lakeArgs) and compares them.
// It returns the C++ action reasons counted per config.
func goVsCppEquivalence(t *testing.T, bin, playbook string, lakeArgs, configs []string) []map[string]int {
	t.Helper()
	_ = os.Setenv("VENUE_SIMULATE", "1")
	dataDir := t.TempDir()
	genArgs := append([]string{"run", "./src/calculation/testdata/genlakefixture", "-bench", dataDir, "-bars", "1500"}, lakeArgs...)
	gen := exec.Command("go", genArgs...)
	gen.Dir = filepath.Join("..", "..")
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("lake fixture: %v\n%s", err, out)
	}
	settings := store.DefaultExperimentSettings()
	from := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC).UnixMilli()
	to := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	settings.FromMs, settings.ToMs = from, to
	applyExperimentEnv(dataDir, settings)
	aggMS := signalstore.AggregationDurationMS()
	startCash := settings.InitialUSDC + settings.InitialUSDT
	nowMS := time.Now().UnixMilli()

	rows, err := tirunner.BuildKPIRange(dataDir, "BTCUSDT", from, to, nowMS)
	if err != nil {
		t.Fatal(err)
	}
	bodies := make(map[int64]string, len(rows))
	var closes []int64
	for _, r := range rows {
		bodies[r.BarOpen] = r.Body
		closes = append(closes, r.BarOpen+aggMS)
	}
	win, err := kpi.LoadWindowBodies(bodies)
	if err != nil {
		t.Fatal(err)
	}
	br, err := tirunner.BuildBarRange(dataDir, "BTCUSDT", from, to, nowMS)
	if err != nil {
		t.Fatal(err)
	}
	var replayBars []signalrunner.StrategyKPIBar
	for _, b := range br.Bars {
		if b.OpenMS >= br.EvalFromMs {
			replayBars = append(replayBars, signalrunner.StrategyKPIBar{CloseMs: b.OpenMS + aggMS, Close: b.Close})
		}
	}
	if len(replayBars) != len(closes) {
		t.Fatalf("bar window mismatch: %d bars vs %d indicator rows", len(replayBars), len(closes))
	}

	items := make([]SweepItemSpec, len(configs))
	for i, c := range configs {
		items[i] = SweepItemSpec{Key: c, Playbook: playbook, StrategyConfig: []byte(c)}
	}
	req, err := BuildSweepRequest(settings, "BTC", "USDT", br, startCash, items)
	if err != nil {
		t.Fatal(err)
	}
	reports, err := RunSweep(context.Background(), bin, req)
	if err != nil {
		t.Fatal(err)
	}

	counts := make([]map[string]int, len(reports))
	for i, rep := range reports {
		counts[i] = map[string]int{}
		for _, a := range rep.Actions {
			counts[i][a.Reason]++
		}
	}

	for i, cfg := range configs {
		t.Run(cfg, func(t *testing.T) {
			ws := t.TempDir()
			if err := writeExperimentPaperWallet(ws, settings, "BTCUSDT"); err != nil {
				t.Fatal(err)
			}
			wallet, err := paper.LoadWallet(ws)
			if err != nil {
				t.Fatal(err)
			}
			opts := &signalrunner.PipelineOpts{StrategyConfig: []byte(cfg), PlaybookKey: playbook, Slim: true, SkipSampler: true}
			runRep, err := signalrunner.FastSimulatedPipelineOpts(ws, win, playbook, "BTCUSDT", closes, opts, nil)
			if err != nil {
				t.Fatal(err)
			}
			goFills, err := signalrunner.LoadInstanceFills(ws, playbook)
			if err != nil {
				t.Fatal(err)
			}
			goRep := signalrunner.BuildInstanceReport(runRep, playbook, "BTCUSDT", from, to, startCash, goFills, win, closes, aggMS)

			actions, err := ReplayActions(reports[i], aggMS)
			if err != nil {
				t.Fatal(err)
			}
			res := signalrunner.ReplayKPI(signalrunner.KPIJob{
				Key: cfg, Playbook: playbook, Symbol: "BTCUSDT", FromMs: from, ToMs: to, StartCash: startCash,
				Coins: wallet.Coins, Costs: sim.FromEnv(), Policy: marketrunner.PolicyFromEnv(),
				Actions: actions, Bars: replayBars, Run: runRep,
			})
			if res.Err != nil {
				t.Fatal(res.Err)
			}
			if len(goFills) < 2 {
				t.Fatalf("test lake produced only %d Go fills", len(goFills))
			}
			if len(res.Fills) != len(goFills) {
				for k := 0; k < min(len(goFills), len(res.Fills)); k++ {
					if res.Fills[k].CandleCloseMs != goFills[k].CandleCloseMs || res.Fills[k].Side != goFills[k].Side {
						t.Logf("first divergence %d: C++ %+v Go %+v", k, res.Fills[k], goFills[k])
						break
					}
				}
				t.Fatalf("fills: C++ path %d, Go path %d", len(res.Fills), len(goFills))
			}
			for k, g := range goFills {
				c := res.Fills[k]
				if c.CandleCloseMs != g.CandleCloseMs || c.Side != g.Side ||
					!relEq(c.BaseQty, g.BaseQty, 1e-9) || !relEq(c.Price, g.Price, 1e-9) {
					t.Fatalf("fill %d: C++ path %+v, Go path %+v", k, c, g)
				}
			}
			var gj, cj any
			gb, _ := json.Marshal(goRep)
			cb, _ := json.Marshal(res.Report)
			_ = json.Unmarshal(gb, &gj)
			_ = json.Unmarshal(cb, &cj)
			compareJSON(t, "report", gj, cj, 1e-6)
		})
	}
	return counts
}

// TestConsensusGoVsCppEquivalence is TestSmaCrossGoVsCppEquivalence for signal-consensus (#130,
// CALC.30): default, changed-threshold and sized configs over two lakes: 1h
// candles (RSI sees the three closes of a bar) and 1h candles with every 17th candle missing
// (a missing hour drops its 3h bar, and the bar after a gap is not traded). Tolerances as above.
// A 3h-candle lake is not used: rsi-3h is never ready on it, so neither side trades.
// The configs must fire vol_cut and pyramid on every lake, or the test proves too little.
func TestConsensusGoVsCppEquivalence(t *testing.T) {
	bin := cppBinaryFor(t, "CPP_SIGNAL_CONSENSUS_BIN", "consensus", "signal-consensus")
	configs := []string{
		`{}`,
		`{"rsiBull":55,"rsiBear":45,"atrStopMult":1.5,"minBarsFlatAfterSell":2,"minBarsLongBeforeSell":1}`,
		`{"volRatioBull":0.95,"volRatioBear":0.9}`,
		`{"sizing":{"unitRiskPct":1,"heatPct":2,"maxUnits":1}}`,
		`{"atrStopMult":1.5,"sizing":{"unitRiskPct":0.5,"maxUnits":3}}`,
		`{"rsiBull":55,"rsiBear":45,"minBarsFlatAfterSell":1,"minBarsLongBeforeSell":1,"sizing":{"unitRiskPct":0.5,"heatPct":1,"maxUnits":4,"addAtrMult":0.25}}`,
		`{"atrStopMult":2,"minBarsFlatAfterSell":1,"minBarsLongBeforeSell":1,"sizing":{"unitRiskPct":0.5,"heatPct":0.5,"maxUnits":3,"addAtrMult":0.5}}`,
	}
	lakes := map[string][]string{
		"1h":          {"-seed", "7", "-hourly"},
		"1h-with-gap": {"-seed", "7", "-hourly", "-gap-every", "17"},
	}
	for _, name := range []string{"1h", "1h-with-gap"} {
		t.Run(name, func(t *testing.T) {
			counts := goVsCppEquivalence(t, bin, "signal-consensus", lakes[name], configs)
			total := map[string]int{}
			for i, c := range counts {
				t.Logf("config %d rules: %v", i, c)
				for k, v := range c {
					total[k] += v
				}
			}
			if total["vol_cut"] == 0 || total["pyramid"] == 0 {
				t.Fatalf("configs must fire vol_cut and pyramid, fired %v", total)
			}
		})
	}
}
