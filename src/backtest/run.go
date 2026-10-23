package backtest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	signalrunner "github.com/MaxBernstetter/trading-copilot/src/calculation-signal/runner"
	tirunner "github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/runner"
	"github.com/MaxBernstetter/trading-copilot/src/backtest/store"
	"github.com/MaxBernstetter/trading-copilot/src/internal/kpi"
	"github.com/MaxBernstetter/trading-copilot/src/internal/signalstore"
	ingestrunner "github.com/MaxBernstetter/trading-copilot/src/ingest/runner"
	"github.com/MaxBernstetter/trading-copilot/src/shared"
)

// Run executes one catalogued backtest by id (DATA_DIR = repo data root).
func Run(dataDir, backtestID string) error {
	if dataDir == "" {
		return fmt.Errorf("DATA_DIR required")
	}
	if backtestID == "" {
		return fmt.Errorf("BACKTEST_ID required")
	}
	_ = os.Setenv("VENUE_SIMULATE", "1")

	cat, err := store.Open(dataDir)
	if err != nil {
		return err
	}
	defer cat.Close()

	rec, err := cat.Get(backtestID)
	if err != nil {
		return err
	}
	fail := func(e error) error {
		_ = cat.MarkFailed(backtestID, e.Error())
		return e
	}
	settings := rec.Settings
	if settings.Algo == "" {
		settings.Algo = "v1"
	}
	if err := signalrunner.ResolvePlaybook(settings.Playbook); err != nil {
		return fail(fmt.Errorf("%w — rebuild image: docker build -f docker/backtest/Dockerfile -t trading-copilot-backtest .", err))
	}
	if err := cat.MarkRunning(backtestID, 0); err != nil {
		return fail(fmt.Errorf("catalog progress: %w", err))
	}
	stage := func(n int, label string, done, total int) {
		_ = cat.SetRunProgress(backtestID, n, label, done, total)
	}

	stage(1, store.StageLabelPrepare, 0, 1)
	ws := store.WorkspaceDir(dataDir, rec)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		return fail(err)
	}
	if err := writePaperWallet(ws, settings); err != nil {
		return fail(err)
	}
	applySettingsEnv(dataDir, settings)
	stage(1, store.StageLabelPrepare, 1, 1)

	nowMS := time.Now().UnixMilli()
	from := settings.FromMs
	to := settings.ToMs
	if to > nowMS {
		to = nowMS
	}
	to -= to % (60 * 60 * 1000)

	runIngest := func(force bool) error {
		stage(2, store.StageLabelMarketData, 0, 1)
		args := []string{
			"-from", fmt.Sprint(from),
			"-to", fmt.Sprint(to),
			"-symbol", settings.Symbol,
		}
		if force {
			args = append(args, "-force")
		}
		code, detail := ingestrunner.Run(args, os.Getenv)
		if code != 0 && code != 2 {
			return fmt.Errorf("ingest failed exit=%d detail=%d", code, detail)
		}
		stage(2, store.StageLabelMarketData, 1, 1)
		return nil
	}

	if err := runIngest(settings.RedownloadMissing); err != nil {
		if settings.RedownloadMissing {
			return fail(fmt.Errorf("market data re-download failed: %w", err))
		}
	}

	stage(3, store.StageLabelIndicators, 0, 1)
	rows, err := tirunner.BuildKPIRange(dataDir, settings.Symbol, from, to, nowMS)
	if err != nil {
		if tirunner.IsMissingParquet(err) && !settings.RedownloadMissing {
			_ = runIngest(false)
			stage(3, store.StageLabelIndicators, 0, 1)
			rows, err = tirunner.BuildKPIRange(dataDir, settings.Symbol, from, to, nowMS)
		}
		if err != nil {
			if tirunner.IsMissingParquet(err) {
				return fail(fmt.Errorf(
					"parquet does not cover the full range (from %s to %s UTC): %w — set From to the first date you have in the lake, or enable force re-download with network",
					formatUTC(from), formatUTC(to), err,
				))
			}
			return fail(fmt.Errorf("indicators: %w", err))
		}
	}
	if err := cat.UpsertKPIRows(backtestID, rows); err != nil {
		return fail(err)
	}
	stage(3, store.StageLabelIndicators, 1, 1)

	aggMS := signalstore.AggregationDurationMS()
	minOpen := from
	maxOpen := to - aggMS
	barOpens, err := cat.ListBarOpens(backtestID, settings.Symbol, settings.Algo, minOpen, maxOpen)
	if err != nil {
		return err
	}
	closes := make([]int64, 0, len(barOpens))
	for _, bo := range barOpens {
		closeMS := bo + aggMS
		if closeMS < from+aggMS || closeMS > to {
			continue
		}
		closes = append(closes, closeMS)
	}
	if len(closes) == 0 {
		return fail(fmt.Errorf("no bars in range"))
	}

	stage(4, store.StageLabelSimulation, 0, len(closes))

	bodies, err := cat.LoadKPIBodies(backtestID, settings.Symbol, settings.Algo, minOpen, maxOpen)
	if err != nil {
		return fail(err)
	}
	kpiWin, err := kpi.LoadWindowBodies(bodies)
	if err != nil {
		return fail(err)
	}

	playbookEnv := settings.Playbook
	totalBars := len(closes)
	onBar := func(done, total int) {
		_ = cat.SetRunProgress(backtestID, 4, store.StageLabelSimulation, done, total)
	}
	if err := signalrunner.FastSimulatedPipeline(ws, kpiWin, playbookEnv, settings.Symbol, closes, onBar); err != nil {
		return fail(err)
	}
	stage(4, store.StageLabelSimulation, totalBars, totalBars)
	return cat.MarkCompleted(backtestID)
}

func writePaperWallet(ws string, s store.Settings) error {
	base := "BTC"
	if s.Symbol == "ETHUSDT" {
		base = "ETH"
	}
	doc := map[string]map[string]float64{
		"coins": {
			"USDC": s.InitialUSDC,
			"USDT": s.InitialUSDT,
			base:   s.InitialBase,
		},
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(ws, "paper_venue.json"), raw, 0o644)
}

const signalConsensusAggMS = 6 * 60 * 60 * 1000

func formatUTC(ms int64) string {
	return time.UnixMilli(ms).UTC().Format("2006-01-02")
}

func applySettingsEnv(dataDir string, s store.Settings) {
	_ = os.Setenv("DATA_DIR", dataDir)
	playbook := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(s.Playbook), ".yaml"), ".yml")
	if playbook == "signal-consensus" {
		_ = os.Setenv("AGGREGATION_DURATION_MS", strconv.FormatInt(signalConsensusAggMS, 10))
	} else {
		_ = os.Unsetenv("AGGREGATION_DURATION_MS")
	}
	if strings.TrimSpace(os.Getenv("BINANCE_BASE_URL")) == "" {
		_ = os.Setenv("BINANCE_BASE_URL", "https://data-api.binance.vision")
	}
	_ = os.Setenv("ALLOC_TARGET_WEIGHT_PCT", strconv.FormatFloat(s.TargetWeightPct, 'f', -1, 64))
	_ = os.Setenv("ALLOC_CASH_RESERVE_QUOTE", strconv.FormatFloat(s.CashReserveQuote, 'f', -1, 64))
	_ = os.Setenv("ALLOC_MIN_ORDER_QUOTE", strconv.FormatFloat(s.MinOrderQuote, 'f', -1, 64))
	_ = os.Setenv("SIM_SPREAD_BPS", strconv.FormatFloat(s.SpreadBps, 'f', -1, 64))
	_ = os.Setenv("SIM_SLIPPAGE_BPS", strconv.FormatFloat(s.SlippageBps, 'f', -1, 64))
	_ = os.Setenv("SIM_FEE_BPS", strconv.FormatFloat(s.FeeBps, 'f', -1, 64))
	_ = os.Setenv("PLAYBOOK", s.Playbook)
	_ = shared.EnsureDataLayout(dataDir)
}
