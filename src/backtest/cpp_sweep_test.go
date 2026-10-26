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
	if p := os.Getenv("CPP_SMA_CROSS_BIN"); p != "" {
		return p
	}
	for _, preset := range []string{"optimized", "debug"} {
		p, _ := filepath.Abs(filepath.Join("..", "..", "build", preset, "strategies", "sma-cross", "strategy-sma-cross"))
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("strategy-sma-cross not built (task cpp:build) and CPP_SMA_CROSS_BIN unset")
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
	_ = os.Setenv("VENUE_SIMULATE", "1")
	dataDir := t.TempDir()
	gen := exec.Command("go", "run", "./src/calculation/testdata/genlakefixture", "-bench", dataDir, "-bars", "1500", "-seed", "7")
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
	items := make([]SweepItemSpec, len(configs))
	for i, c := range configs {
		items[i] = SweepItemSpec{Key: c, Playbook: "ma-cross", StrategyConfig: []byte(c)}
	}
	req, err := BuildSweepRequest(settings, "BTC", "USDT", br, startCash, items)
	if err != nil {
		t.Fatal(err)
	}
	reports, err := RunSweep(context.Background(), bin, req)
	if err != nil {
		t.Fatal(err)
	}

	rules := map[string]int{}
	for _, rep := range reports {
		for _, a := range rep.Actions {
			rules[a.Reason]++
		}
	}
	if rules["vol_cut"] == 0 || rules["pyramid"] == 0 {
		t.Fatalf("configs must fire vol_cut and pyramid, fired %v", rules)
	}
	t.Logf("rules fired: %v", rules)

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
			opts := &signalrunner.PipelineOpts{StrategyConfig: []byte(cfg), PlaybookKey: "ma-cross", Slim: true, SkipSampler: true}
			runRep, err := signalrunner.FastSimulatedPipelineOpts(ws, win, "ma-cross", "BTCUSDT", closes, opts, nil)
			if err != nil {
				t.Fatal(err)
			}
			goFills, err := signalrunner.LoadInstanceFills(ws, "ma-cross")
			if err != nil {
				t.Fatal(err)
			}
			goRep := signalrunner.BuildInstanceReport(runRep, "ma-cross", "BTCUSDT", from, to, startCash, goFills, win, closes, aggMS)

			actions, err := ReplayActions(reports[i], aggMS)
			if err != nil {
				t.Fatal(err)
			}
			res := signalrunner.ReplayKPI(signalrunner.KPIJob{
				Key: cfg, Playbook: "ma-cross", Symbol: "BTCUSDT", FromMs: from, ToMs: to, StartCash: startCash,
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
}
