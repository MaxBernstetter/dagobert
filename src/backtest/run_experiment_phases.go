package backtest

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	"github.com/MaxBernstetter/dagobert/src/calculation-performance"
	tirunner "github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/runner"
	"github.com/MaxBernstetter/dagobert/src/internal/instruments"
	"github.com/MaxBernstetter/dagobert/src/internal/kpi"
	"github.com/MaxBernstetter/dagobert/src/internal/signalstore"
	"github.com/MaxBernstetter/dagobert/src/internal/workspace"
	"github.com/MaxBernstetter/dagobert/src/shared"
)

func experimentRetryFailed() bool {
	return strings.TrimSpace(os.Getenv("EXPERIMENT_RETRY_FAILED")) == "1"
}

func normalizeExperimentSettings(settings store.ExperimentSettings) store.ExperimentSettings {
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
	return settings
}

func loadExperimentWork(cat *experimentRunCatalog, experimentID string, retryFailed bool) (store.ExperimentRecord, []store.ExperimentInstance, error) {
	rec, err := cat.GetExperiment(experimentID)
	if err != nil {
		return store.ExperimentRecord{}, nil, err
	}
	if len(rec.Instances) == 0 {
		return rec, nil, fmt.Errorf("experiment has no instances")
	}
	settings := normalizeExperimentSettings(rec.Settings)
	rec.Settings = settings
	var work []store.ExperimentInstance
	if retryFailed {
		for _, inst := range rec.Instances {
			switch inst.Status {
			case store.StatusFailed, store.StatusRunning:
				work = append(work, inst)
			}
		}
		if len(work) == 0 {
			return rec, nil, fmt.Errorf("no failed instances to retry")
		}
	} else {
		work = rec.Instances
	}
	return rec, work, nil
}

func markExperimentWorkRunning(cat *experimentRunCatalog, experimentID string, rec store.ExperimentRecord, work []store.ExperimentInstance, retryFailed bool) error {
	if retryFailed {
		stillFailed := 0
		for _, inst := range rec.Instances {
			if inst.Status == store.StatusFailed {
				stillFailed++
			}
		}
		if stillFailed > 0 {
			if _, err := cat.MarkExperimentRetryFailed(experimentID); err != nil {
				return fmt.Errorf("catalog progress: %w", err)
			}
			return nil
		}
		return cat.MarkExperimentRunningKeepCompleted(experimentID, len(work))
	}
	return cat.MarkExperimentRunning(experimentID)
}

// RunExperimentPrepare runs stages 1–3 and writes KPI + instance list cache.
func RunExperimentPrepare(dataDir, experimentID string) error {
	if dataDir == "" {
		return fmt.Errorf("DATA_DIR required")
	}
	if experimentID == "" {
		return fmt.Errorf("EXPERIMENT_ID required")
	}
	_ = os.Setenv("VENUE_SIMULATE", "1")

	cat, err := openExperimentRunCatalog()
	if err != nil {
		return err
	}
	defer cat.Close()

	retryFailed := experimentRetryFailed()
	rec, work, err := loadExperimentWork(cat, experimentID, retryFailed)
	if err != nil {
		return err
	}
	settings := rec.Settings
	fail := func(e error) error {
		_ = cat.MarkExperimentFailed(experimentID, e.Error())
		return e
	}
	if err := markExperimentWorkRunning(cat, experimentID, rec, work, retryFailed); err != nil {
		return fail(fmt.Errorf("catalog progress: %w", err))
	}

	instTotal := len(work)
	stage := func(n int, label string, instDone, total int) {
		_ = cat.SetExperimentProgress(experimentID, n, label, instDone, total)
	}

	stage(1, store.ExperimentStageLabelPrepare, 0, instTotal)
	applyExperimentEnv(dataDir, settings)
	stage(1, store.ExperimentStageLabelPrepare, 0, instTotal)

	nowMS := time.Now().UnixMilli()
	from := settings.FromMs
	to := settings.ToMs
	if to > nowMS {
		to = nowMS
	}
	to -= to % (60 * 60 * 1000)

	symbols := experimentSymbols(work, settings)
	failedSymbols := map[string]string{}
	failSymbol := func(sym, msg string) {
		if _, seen := failedSymbols[sym]; seen {
			return
		}
		failedSymbols[sym] = msg
		for _, inst := range work {
			if store.InstanceSymbol(inst, settings) != sym {
				continue
			}
			_ = cat.MarkExperimentInstanceFailed(experimentID, inst.ID, msg, store.InstanceRunTiming{})
		}
	}

	unavailTotal := 0
	stage(2, store.ExperimentStageLabelMarketData, 0, instTotal)
	for _, sym := range symbols {
		base, quote, ok := shared.SplitPipelineSymbol(sym)
		if !ok {
			base, quote = settings.Base, settings.Quote
		}
		cov, err := CheckRawCoverage(dataDir, settings.Provider, base, quote, from, to)
		if err != nil {
			failSymbol(sym, fmt.Sprintf("coverage: %v", err))
			continue
		}
		if !cov.Ready {
			if err := ingestExperimentRange(settings.Provider, base, quote, from, to, settings.RedownloadMissing); err != nil {
				failSymbol(sym, err.Error())
				continue
			}
			cov, err = CheckRawCoverage(dataDir, settings.Provider, base, quote, from, to)
			if err != nil {
				failSymbol(sym, fmt.Sprintf("coverage re-check: %v", err))
				continue
			}
			if !cov.Ready {
				msg := fmt.Sprintf(
					"parquet incomplete for %s [%s, %s): %d retryable closed hours still missing after ingest (unavailable=%d)",
					sym, formatUTC(from), formatUTC(to), len(cov.MissingHours), len(cov.UnavailableHours),
				)
				if sample := formatCoverageGapSample(cov.MissingHours, 5); sample != "" {
					msg += "; e.g. " + sample
				}
				msg += " — check network/rate limits or enable force re-download"
				failSymbol(sym, msg)
				continue
			}
		}
		unavailTotal += len(cov.UnavailableHours)
	}
	if unavailTotal > 0 {
		stage(2, fmt.Sprintf("%s (unavailable=%d)", store.ExperimentStageLabelMarketData, unavailTotal), 0, instTotal)
	} else {
		stage(2, store.ExperimentStageLabelMarketData, 0, instTotal)
	}

	stage(3, store.ExperimentStageLabelIndicators, 0, instTotal)
	bySymbol := make(map[string]experimentSymKPI, len(symbols))
	aggMS := signalstore.AggregationDurationMS()
	for _, sym := range symbols {
		if _, bad := failedSymbols[sym]; bad {
			continue
		}
		rows, err := tirunner.BuildKPIRange(dataDir, sym, from, to, nowMS)
		if err != nil {
			if tirunner.IsMissingParquet(err) {
				failSymbol(sym, fmt.Sprintf("indicators: lake gap after coverage gate: %v", err))
			} else {
				failSymbol(sym, fmt.Sprintf("indicators: %v", err))
			}
			continue
		}
		bodies := make(map[int64]string, len(rows))
		for _, row := range rows {
			bodies[row.BarOpen] = row.Body
		}
		kpiWin, err := kpi.LoadWindowBodies(bodies)
		if err != nil {
			failSymbol(sym, fmt.Sprintf("kpi window: %v", err))
			continue
		}
		closes := make([]int64, 0, len(rows))
		for _, row := range rows {
			closeMS := row.BarOpen + aggMS
			if closeMS < from+aggMS || closeMS > to {
				continue
			}
			closes = append(closes, closeMS)
		}
		if len(closes) == 0 {
			failSymbol(sym, "no bars in range")
			continue
		}
		bySymbol[sym] = experimentSymKPI{win: kpiWin, closes: closes, rows: rows}
	}
	stage(3, store.ExperimentStageLabelIndicators, 0, instTotal)

	if err := WriteAllExperimentSymKPI(dataDir, experimentID, bySymbol); err != nil {
		return fail(err)
	}
	ids := make([]string, len(work))
	for i, inst := range work {
		ids[i] = inst.ID
	}
	meta := experimentPrepareMeta{
		ExperimentID:  experimentID,
		From:          from,
		To:            to,
		NowMS:         nowMS,
		FailedSymbols: failedSymbols,
		Settings:      settings,
		InstanceIDs:   ids,
		InstTotal:     instTotal,
	}
	if err := WriteExperimentPrepareMeta(dataDir, experimentID, meta); err != nil {
		return fail(err)
	}
	return nil
}

// RunExperimentSimulate runs simulation for the given instance ids (loads prepare cache).
func RunExperimentSimulate(dataDir, experimentID string, instanceIDs []string) error {
	if dataDir == "" {
		return fmt.Errorf("DATA_DIR required")
	}
	if experimentID == "" {
		return fmt.Errorf("EXPERIMENT_ID required")
	}
	if len(instanceIDs) == 0 {
		return fmt.Errorf("no instance ids to simulate")
	}
	_ = os.Setenv("VENUE_SIMULATE", "1")

	meta, err := ReadExperimentPrepareMeta(dataDir, experimentID)
	if err != nil {
		return err
	}
	bySymbol, err := LoadExperimentSymKPI(dataDir, experimentID)
	if err != nil {
		return err
	}
	settings := meta.Settings
	applyExperimentEnv(dataDir, settings)
	failedSymbols := meta.FailedSymbols
	if failedSymbols == nil {
		failedSymbols = map[string]string{}
	}

	cat, err := openExperimentRunCatalog()
	if err != nil {
		return err
	}
	defer cat.Close()

	rec, err := cat.GetExperiment(experimentID)
	if err != nil {
		return err
	}
	want := make(map[string]store.ExperimentInstance, len(instanceIDs))
	for _, id := range instanceIDs {
		want[id] = store.ExperimentInstance{}
	}
	for _, inst := range rec.Instances {
		if _, ok := want[inst.ID]; ok {
			want[inst.ID] = inst
		}
	}
	var work []store.ExperimentInstance
	for _, id := range instanceIDs {
		inst, ok := want[id]
		if !ok || inst.ID == "" {
			return fmt.Errorf("unknown instance id %s", id)
		}
		work = append(work, inst)
	}

	instTotal := meta.InstTotal
	if instTotal < 1 {
		instTotal = len(work)
	}
	stage := func(instDone int) {
		_ = cat.SetExperimentProgress(experimentID, 4, store.ExperimentStageLabelSimulation, instDone, instTotal)
	}
	stage(0)

	// Stage 1+2 (compute + KPI) feed a bounded queue; stage 3 (persistence) drains it in batches.
	// Compute never touches a database. Failures of skipped symbols were already recorded by prepare.
	var compute []store.ExperimentInstance
	var precomputed []instanceResult
	skipped := 0
	for _, inst := range work {
		sym := store.InstanceSymbol(inst, settings)
		if _, bad := failedSymbols[sym]; bad {
			skipped++
			continue
		}
		if _, ok := bySymbol[sym]; !ok {
			precomputed = append(precomputed, instanceResult{
				inst: inst, sym: sym, err: fmt.Errorf("no KPI for symbol %s", sym),
			})
			continue
		}
		compute = append(compute, inst)
	}
	queue := make(chan instanceResult, min(len(work), persistQueueMax))
	persistDone := make(chan struct{})
	go func() {
		defer close(persistDone)
		runPersister(cat, dataDir, experimentID, instTotal, skipped, queue)
	}()
	for _, r := range precomputed {
		queue <- r
	}

	cppSimulate(dataDir, meta, bySymbol, compute, queue)
	if SimulateComputeDoneHook != nil {
		SimulateComputeDoneHook()
	}
	close(queue)
	<-persistDone
	return nil
}

// RunExperimentFinalize persists KPI rows and marks experiment completed/partial/failed.
func RunExperimentFinalize(dataDir, experimentID string) error {
	if dataDir == "" {
		return fmt.Errorf("DATA_DIR required")
	}
	if experimentID == "" {
		return fmt.Errorf("EXPERIMENT_ID required")
	}

	meta, err := ReadExperimentPrepareMeta(dataDir, experimentID)
	if err != nil {
		return err
	}
	cat, err := openExperimentRunCatalog()
	if err != nil {
		return err
	}
	defer cat.Close()

	fail := func(e error) error {
		_ = cat.MarkExperimentFailed(experimentID, e.Error())
		return e
	}

	final, err := cat.GetExperiment(experimentID)
	if err != nil {
		return fail(err)
	}
	okN, failN := 0, 0
	for _, inst := range final.Instances {
		switch inst.Status {
		case store.StatusCompleted:
			okN++
		case store.StatusFailed:
			failN++
		}
	}
	summary := fmt.Sprintf("%d of %d instances completed", okN, len(final.Instances))
	if failN > 0 {
		summary = fmt.Sprintf("%d of %d instances completed, %d failed", okN, len(final.Instances), failN)
		if len(meta.FailedSymbols) > 0 {
			parts := make([]string, 0, len(meta.FailedSymbols))
			for sym, msg := range meta.FailedSymbols {
				parts = append(parts, fmt.Sprintf("%s: %s", sym, msg))
			}
			summary += " — " + strings.Join(parts, "; ")
		}
	}
	if okN == 0 {
		return fail(fmt.Errorf("%s", summary))
	}
	if err := writeExperimentPerformance(cat, dataDir, experimentID); err != nil {
		return fail(err)
	}
	if failN > 0 {
		if err := cat.MarkExperimentPartial(experimentID, summary); err != nil {
			return err
		}
		removeExperimentCache(dataDir, experimentID)
		return nil
	}
	if err := cat.MarkExperimentCompleted(experimentID); err != nil {
		return err
	}
	removeExperimentCache(dataDir, experimentID)
	return nil
}

// removeExperimentCache deletes the prepare cache once the experiment is final. Best effort: the
// results are in Postgres and the Parquet files already.
func removeExperimentCache(dataDir, experimentID string) {
	if err := workspace.DeletePrefix(context.Background(), dataDir, ExperimentCacheKey(experimentID)); err != nil {
		log.Printf("experiment %s: remove cache: %v", experimentID, err)
	}
}

func writeExperimentPerformance(cat *experimentRunCatalog, dataDir, experimentID string) error {
	results, err := cat.pg.LoadCompletedResults(cat.ctx(), experimentID, dataDir)
	if err != nil {
		return err
	}
	return cat.pg.ReplaceExperimentPerformance(cat.ctx(), experimentID, performance.Aggregate(results))
}
