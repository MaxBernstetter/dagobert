package backtest

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	signalrunner "github.com/MaxBernstetter/dagobert/src/calculation-signal/runner"
	tirunner "github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/runner"
	ingestrunner "github.com/MaxBernstetter/dagobert/src/ingest/runner"
	"github.com/MaxBernstetter/dagobert/src/internal/instruments"
	"github.com/MaxBernstetter/dagobert/src/internal/kpi"
	"github.com/MaxBernstetter/dagobert/src/internal/signalstore"
)

// Run executes one catalogued backtest by id (DATA_DIR = repo data root).
func Run(dataDir, backtestID string) error {
	if dataDir == "" {
		return fmt.Errorf("DATA_DIR required")
	}
	if backtestID == "" {
		return fmt.Errorf("BACKTEST_ID required")
	}
	if err := os.Setenv("VENUE_SIMULATE", "1"); err != nil {
		return err
	}

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
		if err := cat.MarkFailed(backtestID, e.Error()); err != nil {
			log.Printf("backtest %s: could not record the failure %q: %v", backtestID, e, err)
		}
		return e
	}
	settings := rec.Settings
	if settings.Algo == "" {
		settings.Algo = "v1"
	}
	if strings.TrimSpace(settings.Provider) == "" {
		settings.Provider = instruments.PriceProvider(settings.PipelineSymbol())
	}
	if strings.TrimSpace(settings.Base) == "" {
		settings.Base = "BTC"
	}
	if strings.TrimSpace(settings.Quote) == "" {
		settings.Quote = "USDT"
	}
	series := settings.PipelineSymbol()
	if err := signalrunner.ResolvePlaybook(settings.Playbook); err != nil {
		return fail(fmt.Errorf("%w — rebuild image: docker build -f docker/backtest/Dockerfile -t dagobert-backtest .", err))
	}
	if err := cat.MarkRunning(backtestID, 0); err != nil {
		return fail(fmt.Errorf("catalog progress: %w", err))
	}
	stage := func(n int, label string, done, total int) {
		if err := cat.SetRunProgress(backtestID, n, label, done, total); err != nil {
			log.Printf("backtest %s: progress not saved: %v", backtestID, err)
		}
	}

	stage(1, store.StageLabelPrepare, 0, 1)
	ws := store.WorkspaceDir(dataDir, rec)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		return fail(err)
	}
	if err := writePaperWallet(ws, settings); err != nil {
		return fail(err)
	}
	if err := applySettingsEnv(dataDir, settings); err != nil {
		return fail(err)
	}
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
			"-base", settings.Base,
			"-quote", settings.Quote,
			"-provider", settings.Provider,
		}
		if force {
			args = append(args, "-force")
		}
		code, detail := ingestrunner.Run(args, os.Getenv)
		if code != 0 && code != 2 {
			hint := "see ingest logs"
			switch detail {
			case 30:
				hint = "Binance transport failed after retries"
			case 32:
				hint = "exchange holes recorded (unexpected with exit 1)"
			case 40:
				hint = "lake/sqlite inconsistency"
			}
			return fmt.Errorf("ingest failed exit=%d detail=%d (%s)", code, detail, hint)
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
	rows, err := tirunner.BuildKPIRange(dataDir, series, from, to, nowMS)
	if err != nil {
		if tirunner.IsMissingParquet(err) && !settings.RedownloadMissing {
			_ = runIngest(false)
			stage(3, store.StageLabelIndicators, 0, 1)
			rows, err = tirunner.BuildKPIRange(dataDir, series, from, to, nowMS)
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
	barOpens, err := cat.ListBarOpens(backtestID, series, settings.Algo, minOpen, maxOpen)
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

	bodies, err := cat.LoadKPIBodies(backtestID, series, settings.Algo, minOpen, maxOpen)
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
		logIfErr("save the progress of backtest "+backtestID, cat.SetRunProgress(backtestID, 4, store.StageLabelSimulation, done, total))
	}
	if _, err := signalrunner.FastSimulatedPipeline(ws, kpiWin, playbookEnv, series, closes, onBar); err != nil {
		return fail(err)
	}
	stage(4, store.StageLabelSimulation, totalBars, totalBars)
	return cat.MarkCompleted(backtestID)
}

func writePaperWallet(ws string, s store.Settings) error {
	coins := paperVenueCoins(s.PipelineSymbol(), s.InitialUSDC+s.InitialUSDT, s.InitialBase)
	doc := map[string]map[string]float64{"coins": coins}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(ws, "paper_venue.json"), raw, 0o644)
}

func formatUTC(ms int64) string {
	return time.UnixMilli(ms).UTC().Format("2006-01-02")
}

func applySettingsEnv(dataDir string, s store.Settings) error {
	return setProcessEnv(dataDir, map[string]string{
		"DATA_DIR":                 dataDir,
		"ALLOC_TARGET_WEIGHT_PCT":  strconv.FormatFloat(s.TargetWeightPct, 'f', -1, 64),
		"ALLOC_CASH_RESERVE_QUOTE": strconv.FormatFloat(s.CashReserveQuote, 'f', -1, 64),
		"ALLOC_MIN_ORDER_QUOTE":    strconv.FormatFloat(s.MinOrderQuote, 'f', -1, 64),
		"SIM_SPREAD_BPS":           strconv.FormatFloat(s.SpreadBps, 'f', -1, 64),
		"SIM_SLIPPAGE_BPS":         strconv.FormatFloat(s.SlippageBps, 'f', -1, 64),
		"SIM_FEE_BPS":              strconv.FormatFloat(s.FeeBps, 'f', -1, 64),
		"PLAYBOOK":                 s.Playbook,
	}, map[string]string{"BINANCE_BASE_URL": "https://data-api.binance.vision"})
}

// logIfErr reports a failed best-effort write (a progress or failure record) instead of dropping it silently.
func logIfErr(what string, err error) {
	if err != nil {
		log.Printf("could not %s: %v", what, err)
	}
}
