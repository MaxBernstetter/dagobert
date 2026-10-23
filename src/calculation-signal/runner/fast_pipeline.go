package runner

import (
	"fmt"

	marketrunner "github.com/MaxBernstetter/trading-copilot/src/calculation-market-action/runner"
	"github.com/MaxBernstetter/trading-copilot/src/calculation-signal/internal/framework"
	"github.com/MaxBernstetter/trading-copilot/src/calculation-signal/internal/playbooks"
	execrunner "github.com/MaxBernstetter/trading-copilot/src/execution/runner"
	"github.com/MaxBernstetter/trading-copilot/src/internal/instruments"
	"github.com/MaxBernstetter/trading-copilot/src/internal/kpi"
	"github.com/MaxBernstetter/trading-copilot/src/internal/signalstore"
	"github.com/MaxBernstetter/trading-copilot/src/internal/venue/paper"
)

// FastSimulatedPipeline replays signal → market-action → execution with KPI window preload
// and batched SQLite writes. workspaceDir holds signal/execution/paper; kpiWin supplies marks.
// Requires VENUE_SIMULATE=1.
// BarProgress is called during simulation with done/total bar counts (may be throttled).
type BarProgress func(done, total int)

func FastSimulatedPipeline(workspaceDir string, kpiWin *kpi.Window, playbookEnv, symbol string, candleCloses []int64, onBar BarProgress) error {
	if !paper.Enabled() {
		return fmt.Errorf("fast pipeline requires VENUE_SIMULATE=1")
	}
	if len(candleCloses) == 0 {
		return nil
	}
	entry, err := playbooks.Resolve(playbookEnv)
	if err != nil {
		return err
	}
	playbookID := entry.ID
	inst, ok := instruments.BySeries(symbol)
	if !ok {
		return fmt.Errorf("unknown symbol %s", symbol)
	}

	aggMS := framework.AggregationDurationMS()
	win := kpiWin
	if win == nil {
		return fmt.Errorf("kpi window required")
	}

	wallet, err := paper.LoadWallet(workspaceDir)
	if err != nil {
		return err
	}

	var signals []signalstore.SignalRow
	var actions []signalstore.ActionRow
	simBatch := execrunner.NewSimBatch()

	position := "flat"
	lastBuyBar := -1
	lastSellBar := -1
	total := len(candleCloses)
	reportEvery := 1
	if total > 200 {
		reportEvery = total / 100
		if reportEvery < 1 {
			reportEvery = 1
		}
	}

	for i, candleClose := range candleCloses {
		barOpen := candleClose - aggMS
		results, err := win.Results(barOpen)
		if err != nil {
			return fmt.Errorf("kpi missing bar_open=%d: %w", barOpen, err)
		}
		barClose, _ := win.Close(barOpen)
		barVolume, _ := win.Volume(barOpen)
		var prevResults []kpi.Result
		var prevBarClose float64
		hasPreviousBar := false
		prevOpen := barOpen - aggMS
		if prevOpen >= 0 {
			prevResults, err = win.Results(prevOpen)
			if err == nil {
				hasPreviousBar = true
				prevBarClose, _ = win.Close(prevOpen)
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
		dec, err := entry.Strategy.Evaluate(framework.Context{
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
		})
		if err != nil {
			return err
		}
		signalJSON, err := playbooks.EncodeSignalJSON(entry, symbol, dec)
		if err != nil {
			return err
		}
		signals = append(signals, signalstore.SignalRow{
			Playbook: playbookID, Symbol: symbol, CandleClose: candleClose, Body: signalJSON,
		})

		mark := barClose
		for _, hr := range []int{3, 6, 12, 24, 48} {
			if m, ok := kpi.MovingAverage(results, "sma", hr); ok && m > 0 {
				mark = m
				break
			}
		}
		if mark <= 0 {
			mark = 1
		}
		quoteCash, baseHeld := wallet.QuoteAndBase(inst.Quote, inst.Base)
		planned, err := marketrunner.PlanAction(playbookID, symbol, dec.Decision, dec.RuleID, quoteCash, baseHeld, mark)
		if err != nil {
			return err
		}
		actions = append(actions, signalstore.ActionRow{
			Playbook: playbookID, Symbol: symbol, CandleClose: candleClose, Body: planned.Body,
		})

		price, err := win.Close(barOpen)
		if err != nil || price <= 0 {
			price = mark
		}
		if price <= 0 {
			return fmt.Errorf("no bar price bar_open=%d", barOpen)
		}
		step, err := execrunner.SimulateBar(&wallet, playbookID, symbol, candleClose, planned.Decision, planned.Size, price, position)
		if err != nil {
			return err
		}
		simBatch.Add(step)
		position = step.Position
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

	sigDB, err := signalstore.Open(workspaceDir)
	if err != nil {
		return err
	}
	defer sigDB.Close()
	if err := signalstore.ReplaceSignalsAndActions(sigDB, signals, actions); err != nil {
		return err
	}
	return simBatch.Flush(workspaceDir, &wallet)
}
