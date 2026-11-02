package backtest

import (
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	marketrunner "github.com/MaxBernstetter/dagobert/src/calculation-market-action/runner"
	signalrunner "github.com/MaxBernstetter/dagobert/src/calculation-signal/runner"
	tirunner "github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/runner"
	"github.com/MaxBernstetter/dagobert/src/internal/signalstore"
	"github.com/MaxBernstetter/dagobert/src/internal/venue/paper"
	"github.com/MaxBernstetter/dagobert/src/internal/venue/sim"
)

// The experiment engine is C++ only. These tests run the same path as cppSimulate (sweep -> actions ->
// ReplayKPI) over seeded lakes and check what the replay hands to the persister: one journal and one fill
// order per fill, the same values in both, and period statistics that add up to the trade outcomes.

// cppRun is one config replayed on one lake.
type cppRun struct {
	cfg     string
	res     signalrunner.KPIResult
	reasons map[string]int // C++ action reasons (rules fired)
	sells   int
}

// runCppLake sweeps every config on one seeded lake and replays the actions.
func runCppLake(t *testing.T, bin, playbook string, lakeArgs, configs []string) []cppRun {
	t.Helper()
	t.Setenv("VENUE_SIMULATE", "1")
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
	var closes []int64
	for _, r := range rows {
		closes = append(closes, r.BarOpen+aggMS)
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

	out := make([]cppRun, 0, len(configs))
	for i, cfg := range configs {
		ws := t.TempDir()
		if err := writeExperimentPaperWallet(ws, settings, "BTCUSDT"); err != nil {
			t.Fatal(err)
		}
		wallet, err := paper.LoadWallet(ws)
		if err != nil {
			t.Fatal(err)
		}
		actions, err := ReplayActions(reports[i], aggMS)
		if err != nil {
			t.Fatal(err)
		}
		run := cppRun{cfg: cfg, reasons: map[string]int{}}
		for _, a := range reports[i].Actions {
			run.reasons[a.Reason]++
		}
		run.res = signalrunner.ReplayKPI(signalrunner.KPIJob{
			Key: cfg, Playbook: playbook, Symbol: "BTCUSDT", FromMs: from, ToMs: to, StartCash: startCash,
			Coins: wallet.Coins, Costs: sim.FromEnv(), Policy: marketrunner.PolicyFromEnv(),
			Actions: actions, Bars: replayBars,
		})
		if run.res.Err != nil {
			t.Fatalf("config %s: %v", cfg, run.res.Err)
		}
		for _, f := range run.res.Fills {
			if f.Side == "sell" {
				run.sells++
			}
		}
		out = append(out, run)
	}
	return out
}

// checkCppRun asserts the invariants of one replayed config.
func checkCppRun(t *testing.T, run cppRun) {
	t.Helper()
	res := run.res
	if len(res.Journals) != len(res.Fills) {
		t.Fatalf("%s: %d journals for %d fills", run.cfg, len(res.Journals), len(res.Fills))
	}
	var fillOrders []int
	for i, o := range res.Orders {
		if o.Kind == "fill" {
			fillOrders = append(fillOrders, i)
		}
	}
	if len(fillOrders) != len(res.Fills) {
		t.Fatalf("%s: %d fill orders for %d fills", run.cfg, len(fillOrders), len(res.Fills))
	}
	for i, f := range res.Fills {
		o := res.Orders[fillOrders[i]]
		j := res.Journals[i]
		if o.CandleClose != f.CandleCloseMs || o.OrderLinkID != j.OrderLinkID {
			t.Fatalf("%s fill %d: order %s@%d does not match journal %s", run.cfg, i, o.OrderLinkID, o.CandleClose, j.OrderLinkID)
		}
		if strings.ToLower(j.Side) != f.Side {
			t.Fatalf("%s fill %d: side %q vs journal %q", run.cfg, i, f.Side, j.Side)
		}
		if got, _ := strconv.ParseFloat(j.BaseQty, 64); got != f.BaseQty {
			t.Fatalf("%s fill %d: base %v vs journal %q", run.cfg, i, f.BaseQty, j.BaseQty)
		}
		if got, _ := strconv.ParseFloat(j.QuoteQty, 64); got != f.QuoteQty {
			t.Fatalf("%s fill %d: quote %v vs journal %q", run.cfg, i, f.QuoteQty, j.QuoteQty)
		}
		if got, _ := strconv.ParseFloat(j.Price, 64); got != f.Price {
			t.Fatalf("%s fill %d: price %v vs journal %q", run.cfg, i, f.Price, j.Price)
		}
	}
	p := res.Report.Performance
	monthTrades, monthWins, yearTrades, yearWins := 0, 0, 0, 0
	for _, m := range p.Monthly {
		monthTrades += m.Trades
		monthWins += m.Wins
	}
	for _, y := range p.Yearly {
		yearTrades += y.Trades
		yearWins += y.Wins
	}
	if monthTrades != p.ClosedTrades() || yearTrades != p.ClosedTrades() {
		t.Fatalf("%s: period trades %d/%d vs closed %d", run.cfg, monthTrades, yearTrades, p.ClosedTrades())
	}
	if monthWins != p.Wins || yearWins != p.Wins {
		t.Fatalf("%s: period wins %d/%d vs wins %d", run.cfg, monthWins, yearWins, p.Wins)
	}
	if p.Breakeven < 0 || p.Wins+p.Losses != len(p.Trades) {
		t.Fatalf("%s: wins %d losses %d breakeven %d vs %d trades", run.cfg, p.Wins, p.Losses, p.Breakeven, len(p.Trades))
	}
	if p.ClosedTrades() > run.sells {
		t.Fatalf("%s: %d closed trades from %d sells", run.cfg, p.ClosedTrades(), run.sells)
	}
}

func TestSmaCrossCppReplayConsistent(t *testing.T) {
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
	trades := 0
	for _, run := range runCppLake(t, bin, "ma-cross", []string{"-seed", "7"}, configs) {
		checkCppRun(t, run)
		for k, v := range run.reasons {
			rules[k] += v
		}
		trades += run.res.Report.Performance.ClosedTrades()
	}
	if rules["vol_cut"] == 0 || rules["pyramid"] == 0 {
		t.Fatalf("configs must fire vol_cut and pyramid, fired %v", rules)
	}
	if trades == 0 {
		t.Fatal("no config closed a trade: the fixture proves nothing")
	}
	t.Logf("rules fired: %v, closed trades: %d", rules, trades)
}

func TestConsensusCppReplayConsistent(t *testing.T) {
	bin := cppBinaryFor(t, "CPP_SIGNAL_CONSENSUS_BIN", "consensus", "signal-consensus")
	configs := []string{
		`{}`,
		`{"rsiBull":55,"rsiBear":45,"atrStopMult":1.5,"minBarsFlatAfterSell":2,"minBarsLongBeforeSell":1}`,
		`{"volRatioBull":0.95,"volRatioBear":0.9}`,
		`{"sizing":{"unitRiskPct":1,"heatPct":2,"maxUnits":1}}`,
		`{"atrStopMult":1.5,"sizing":{"unitRiskPct":0.5,"maxUnits":3}}`,
		`{"rsiBull":55,"rsiBear":45,"minBarsFlatAfterSell":1,"minBarsLongBeforeSell":1,"sizing":{"unitRiskPct":0.5,"heatPct":1,"maxUnits":4,"addAtrMult":0.25}}`,
		`{"atrStopMult":2,"minBarsFlatAfterSell":1,"minBarsLongBeforeSell":1,"sizing":{"unitRiskPct":0.5,"heatPct":0.5,"maxUnits":3,"addAtrMult":0.5}}`,
		// Explicit zero cooldowns (#128): read as 0, not as the default 8/4.
		`{"minBarsFlatAfterSell":0,"minBarsLongBeforeSell":0}`,
		`{"minBarsFlatAfterSell":0}`,
		`{"minBarsLongBeforeSell":0,"sizing":{"unitRiskPct":0.5,"heatPct":1,"maxUnits":3}}`,
	}
	lakes := map[string][]string{
		"1h":          {"-seed", "7", "-hourly"},
		"1h-with-gap": {"-seed", "7", "-hourly", "-gap-every", "17"},
	}
	for _, name := range []string{"1h", "1h-with-gap"} {
		t.Run(name, func(t *testing.T) {
			rules := map[string]int{}
			for _, run := range runCppLake(t, bin, "signal-consensus", lakes[name], configs) {
				checkCppRun(t, run)
				for k, v := range run.reasons {
					rules[k] += v
				}
			}
			if rules["vol_cut"] == 0 || rules["pyramid"] == 0 {
				t.Fatalf("configs must fire vol_cut and pyramid, fired %v", rules)
			}
		})
	}
}
