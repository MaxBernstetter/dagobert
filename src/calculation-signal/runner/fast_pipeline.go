package runner

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	marketrunner "github.com/MaxBernstetter/dagobert/src/calculation-market-action/runner"
	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/playbooks"
	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/sizing"
	macross "github.com/MaxBernstetter/dagobert/src/calculation-signal/playbooks/ma-cross"
	signalconsensus "github.com/MaxBernstetter/dagobert/src/calculation-signal/playbooks/signal-consensus"
	execrunner "github.com/MaxBernstetter/dagobert/src/execution/runner"
	"github.com/MaxBernstetter/dagobert/src/internal/instruments"
	"github.com/MaxBernstetter/dagobert/src/internal/kpi"
	"github.com/MaxBernstetter/dagobert/src/internal/signalstore"
	"github.com/MaxBernstetter/dagobert/src/internal/venue/paper"
	"github.com/MaxBernstetter/dagobert/src/internal/venue/sim"
)

// BarProgress is called during simulation with done/total bar counts (may be throttled).
type BarProgress func(done, total int)

// PipelineOpts customizes FastSimulatedPipeline (nil = defaults).
type PipelineOpts struct {
	// StrategyConfig is playbook-specific JSON (ma-cross windows; signal-consensus thresholds).
	StrategyConfig json.RawMessage
	// PlaybookKey overrides entry.ID for signal/action/fill identity when set.
	PlaybookKey string
	// Slim skips signal/action SQLite, no-trade order rows, and per-bar JSON encode (experiment path).
	Slim bool
	// SkipSampler disables the resource sampler (faster experiment sims).
	SkipSampler bool
}

// RunReport is the wall-clock + optional resource report for one pipeline run.
type RunReport = framework.RunReport

// MetricStats is avg/med/p95 over in-run samples (re-exported).
type MetricStats = framework.MetricStats

// StageTimings holds optional profile stages (EXPERIMENT_SIM_PROFILE=1).
type StageTimings struct {
	LoopMs   int64 `json:"loopMs"`
	EncodeMs int64 `json:"encodeMs,omitempty"`
	FlushMs  int64 `json:"flushMs"`
}

// FastSimulatedPipeline replays signal → market-action → execution with KPI window preload
// and batched SQLite writes. workspaceDir holds signal/execution/paper; kpiWin supplies marks.
// Requires VENUE_SIMULATE=1.
func FastSimulatedPipeline(workspaceDir string, kpiWin *kpi.Window, playbookEnv, symbol string, candleCloses []int64, onBar BarProgress) (RunReport, error) {
	return FastSimulatedPipelineOpts(workspaceDir, kpiWin, playbookEnv, symbol, candleCloses, nil, onBar)
}

// FastSimulatedPipelineOpts is FastSimulatedPipeline with optional strategy config / playbook key.
func FastSimulatedPipelineOpts(workspaceDir string, kpiWin *kpi.Window, playbookEnv, symbol string, candleCloses []int64, opts *PipelineOpts, onBar BarProgress) (RunReport, error) {
	start := time.Now()
	slim := opts != nil && opts.Slim
	skipSampler := (opts != nil && opts.SkipSampler) || slim
	profile := profileEnabled()

	var sampler *resourceSampler
	if !skipSampler {
		sampler = startResourceSampler(resourceSampleInterval)
	}

	var stages StageTimings
	finish := func(err error) (RunReport, error) {
		if sampler != nil {
			sampler.Stop()
		}
		report := framework.NewRunReport(start, time.Now())
		if sampler != nil {
			sampler.Apply(&report)
		}
		if profile {
			logProfile(workspaceDir, stages, report.DurationMs)
		}
		return report, err
	}

	if !paper.Enabled() {
		return finish(fmt.Errorf("fast pipeline requires VENUE_SIMULATE=1"))
	}
	if len(candleCloses) == 0 {
		return finish(nil)
	}
	entry, err := playbooks.Resolve(playbookEnv)
	if err != nil {
		return finish(err)
	}

	var strategyConfig json.RawMessage
	playbookID := entry.ID
	if opts != nil {
		strategyConfig = opts.StrategyConfig
		if opts.PlaybookKey != "" {
			playbookID = opts.PlaybookKey
		}
	}

	var configJSON []byte
	aggMS := framework.AggregationDurationMS()
	switch entry.Strategy.Slug() {
	case "ma-cross":
		cfg, err := macross.ParseConfig(strategyConfig)
		if err != nil {
			return finish(err)
		}
		if err := cfg.Validate(aggMS); err != nil {
			return finish(err)
		}
		configJSON, err = cfg.CanonicalJSON()
		if err != nil {
			return finish(err)
		}
		strategyConfig = configJSON
		if opts != nil && opts.PlaybookKey == "" && len(opts.StrategyConfig) > 0 {
			hash, err := cfg.Hash()
			if err != nil {
				return finish(err)
			}
			playbookID = playbooks.InstancePlaybookKey(entry.ID, hash)
		}
	case "signal-consensus":
		cfg, err := signalconsensus.ParseConfig(strategyConfig)
		if err != nil {
			return finish(err)
		}
		if err := cfg.Validate(aggMS); err != nil {
			return finish(err)
		}
		configJSON, err = cfg.CanonicalJSON()
		if err != nil {
			return finish(err)
		}
		strategyConfig = configJSON
		if opts != nil && opts.PlaybookKey == "" && len(opts.StrategyConfig) > 0 {
			hash, err := cfg.Hash()
			if err != nil {
				return finish(err)
			}
			playbookID = playbooks.InstancePlaybookKey(entry.ID, hash)
		}
	}

	inst, ok := instruments.Resolve(symbol)
	if !ok {
		return finish(fmt.Errorf("unknown symbol %s", symbol))
	}

	win := kpiWin
	if win == nil {
		return finish(fmt.Errorf("kpi window required"))
	}

	wallet, err := paper.LoadWallet(workspaceDir)
	if err != nil {
		return finish(err)
	}

	pol := marketrunner.PolicyFromEnv()
	costs := sim.FromEnv()
	encodeBodies := !slim

	var signals []signalstore.SignalRow
	var actions []signalstore.ActionRow
	simBatch := execrunner.NewSimBatch()

	position := "flat"
	lastBuyBar := -1
	lastSellBar := -1
	var openUnits []sizing.Unit
	total := len(candleCloses)
	reportEvery := 1
	if total > 200 {
		reportEvery = total / 100
		if reportEvery < 1 {
			reportEvery = 1
		}
	}

	loopStart := time.Now()
	var encodeDur time.Duration

	for i, candleClose := range candleCloses {
		barOpen := candleClose - aggMS
		snap, ok := win.Snapshot(barOpen)
		if !ok {
			return finish(fmt.Errorf("kpi missing bar_open=%d", barOpen))
		}
		results := snap.Results
		barClose := snap.Close
		barVolume := snap.Volume
		var prevResults []kpi.Result
		var prevBarClose float64
		hasPreviousBar := false
		prevOpen := barOpen - aggMS
		if prevOpen >= 0 {
			if prev, ok := win.Snapshot(prevOpen); ok {
				hasPreviousBar = true
				prevResults = prev.Results
				prevBarClose = prev.Close
			}
		}
		var previousBarOpen int64
		gapFromPrevious := false
		if i > 0 {
			previousBarOpen = candleCloses[i-1] - aggMS
			if barOpen-previousBarOpen > aggMS {
				gapFromPrevious = true
			}
		}

		barsSinceBuy := 999
		barsSinceSell := 999
		if lastBuyBar >= 0 {
			barsSinceBuy = i - lastBuyBar
		}
		if lastSellBar >= 0 {
			barsSinceSell = i - lastSellBar
		}
		var lastFill, equity float64
		if len(openUnits) > 0 {
			lastFill = openUnits[len(openUnits)-1].Fill
		}
		if q0, b0 := wallet.QuoteAndBase(inst.Quote, inst.Base); barClose > 0 {
			equity = q0 + b0*barClose
		}
		dec, err := entry.Strategy.Evaluate(framework.Context{
			Equity:           equity,
			OpenUnits:        append([]sizing.Unit(nil), openUnits...),
			LastFill:         lastFill,
			Symbol:           symbol,
			CandleClose:      candleClose,
			BarOpen:          barOpen,
			BarClose:         barClose,
			BarVolume:        barVolume,
			PreviousBarClose: prevBarClose,
			BarsSinceBuy:     barsSinceBuy,
			BarsSinceSell:    barsSinceSell,
			Position:         position,
			Results:          results,
			PreviousResults:  prevResults,
			HasPreviousBar:   hasPreviousBar,
			PreviousBarOpen:  previousBarOpen,
			GapFromPrevious:  gapFromPrevious,
			StrategyConfig:   strategyConfig,
		})
		if err != nil {
			return finish(err)
		}

		if encodeBodies {
			encStart := time.Now()
			signalJSON, err := playbooks.EncodeSignalJSONWithConfig(entry, symbol, dec, configJSON)
			if err != nil {
				return finish(err)
			}
			signals = append(signals, signalstore.SignalRow{
				Playbook: playbookID, Symbol: symbol, CandleClose: candleClose, Body: signalJSON,
			})
			encodeDur += time.Since(encStart)
		}

		mark := barClose
		for _, hr := range []int{3, 6, 12, 24, 48} {
			if m, ok := kpi.MovingAverageIndexed(snap.ByKey, "sma", hr); ok && m > 0 {
				mark = m
				break
			}
		}
		if mark <= 0 {
			mark = 1
		}
		quoteCash, baseHeld := wallet.QuoteAndBase(inst.Quote, inst.Base)
		planned, err := marketrunner.PlanActionWithPolicy(playbookID, symbol, dec.Decision, dec.RuleID, quoteCash, baseHeld, mark, dec.QuoteNotional, dec.BaseQty, pol, encodeBodies)
		if err != nil {
			return finish(err)
		}
		if encodeBodies {
			actions = append(actions, signalstore.ActionRow{
				Playbook: playbookID, Symbol: symbol, CandleClose: candleClose, Body: planned.Body,
			})
		}

		price := barClose
		if price <= 0 {
			price = mark
		}
		if price <= 0 {
			return finish(fmt.Errorf("no bar price bar_open=%d", barOpen))
		}
		baseBefore := baseHeld
		step, err := execrunner.SimulateBarWithModel(&wallet, playbookID, symbol, candleClose, planned.Decision, planned.Size, price, position, costs, slim)
		if err != nil {
			return finish(err)
		}
		simBatch.Add(step)
		position = step.Position
		if dec.StopPrice != nil || len(openUnits) > 0 {
			_, baseAfter := wallet.QuoteAndBase(inst.Quote, inst.Base)
			openUnits = trackUnits(openUnits, dec, pol.MinOrderQuote, baseBefore, baseAfter, price)
		}
		if len(openUnits) > 0 {
			position = "long"
		}
		switch dec.Decision {
		case "buy":
			lastBuyBar = i
		case "sell":
			lastSellBar = i
		}
		if onBar != nil && (i%reportEvery == 0 || i == total-1) {
			onBar(i+1, total)
		}
	}
	stages.LoopMs = time.Since(loopStart).Milliseconds()
	stages.EncodeMs = encodeDur.Milliseconds()

	flushStart := time.Now()
	if encodeBodies {
		sigDB, err := signalstore.Open(workspaceDir)
		if err != nil {
			return finish(err)
		}
		if err := signalstore.ReplaceSignalsAndActions(sigDB, signals, actions); err != nil {
			sigDB.Close()
			return finish(err)
		}
		sigDB.Close()
	}
	if err := simBatch.Flush(workspaceDir, &wallet); err != nil {
		return finish(err)
	}
	stages.FlushMs = time.Since(flushStart).Milliseconds()
	return finish(nil)
}

func profileEnabled() bool {
	v := strings.TrimSpace(os.Getenv("EXPERIMENT_SIM_PROFILE"))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

func logProfile(workspace string, stages StageTimings, totalMs int64) {
	fmt.Fprintf(os.Stderr, "experiment_sim_profile workspace=%s totalMs=%d loopMs=%d encodeMs=%d flushMs=%d\n",
		workspace, totalMs, stages.LoopMs, stages.EncodeMs, stages.FlushMs)
}

// trackUnits keeps the open sized units in step with simulated fills. Decisions
// without a stop price (legacy sizing) add nothing. A full exit, or a remainder
// worth less than the venue minimum, clears the units.
func trackUnits(units []sizing.Unit, dec framework.Decision, minOrderQuote float64, baseBefore, baseAfter, price float64) []sizing.Unit {
	switch dec.Decision {
	case "buy":
		if dec.RaiseStopsBy != nil && baseAfter > baseBefore {
			for i := range units {
				units[i].Stop += math.Max(0, *dec.RaiseStopsBy)
			}
		}
		if dec.StopPrice != nil && baseAfter > baseBefore {
			units = append(units, sizing.Unit{Fill: price, Stop: *dec.StopPrice, Qty: baseAfter - baseBefore})
		}
	case "sell":
		if dec.BaseQty == nil || baseBefore <= 0 || baseAfter <= 0 || baseAfter*price < minOrderQuote {
			return nil
		}
		keep := baseAfter / baseBefore
		for i := range units {
			units[i].Qty *= keep
		}
	}
	return units
}
