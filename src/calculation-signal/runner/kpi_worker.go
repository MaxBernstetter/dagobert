package runner

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	marketrunner "github.com/MaxBernstetter/dagobert/src/calculation-market-action/runner"
	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
	execrunner "github.com/MaxBernstetter/dagobert/src/execution/runner"
	"github.com/MaxBernstetter/dagobert/src/internal/instruments"
	"github.com/MaxBernstetter/dagobert/src/internal/venue/paper"
	"github.com/MaxBernstetter/dagobert/src/internal/venue/sim"
)

// ReplayAction is one sized order decision of a strategy instance (the C++ sweep outcome).
// Size is the order quantity as the execution venue takes it: quote spend for a buy, base
// amount for a sell. Bars without an action are no-trade and need no entry.
//
// When Sizing is set, Size is ignored and the order is sized at replay time from the replayed
// wallet with the market-action policy (KPIJob.Policy), exactly as the Go pipeline sizes it.
type ReplayAction struct {
	CandleCloseMs int64
	Decision      string // "buy" or "sell"
	Size          float64
	Sizing        *ReplaySizing
}

// ReplaySizing carries the signal-level sizing of an action. Nil QuoteNotional and BaseQty keep
// the legacy sizing (buy up to the target weight, sell everything). Equity is the book equity
// the C++ strategy sized against; when set, both amounts are rescaled by goWalletEquity/Equity
// (a vol_cut with HeatBasePerEquity is recomputed instead),
// where goWalletEquity is the replayed wallet marked to market at the decision bar close.
type ReplaySizing struct {
	QuoteNotional *float64
	BaseQty       *float64
	Equity        float64
	// HeatBasePerEquity marks a vol_cut: the base the heat budget allows per unit of equity.
	// The cut is recomputed on the replayed wallet as held - equity*HeatBasePerEquity, because
	// it is affine in held and equity and a linear rescale would amplify their drift.
	HeatBasePerEquity *float64
}

// KPIJob is one instance to turn into fills and KPIs. Nothing in it touches disk.
type KPIJob struct {
	Key       string // caller's identity for the instance, echoed in the result
	Playbook  string
	Symbol    string
	FromMs    int64
	ToMs      int64
	StartCash float64
	Coins     map[string]float64 // starting paper wallet; copied before use
	Costs     sim.Model
	Policy    marketrunner.Policy // used by actions that carry Sizing
	Actions   []ReplayAction      // ascending CandleCloseMs
	Bars      []StrategyKPIBar
	Run       RunReport
}

// StrategyKPIBar is one mark-to-market sample at a candle close (re-exported).
type StrategyKPIBar = framework.StrategyKPIBar

// KPIResult is the outcome of one KPIJob.
type KPIResult struct {
	Key    string
	Fills  []StrategyKPIFill
	Report InstanceReport
	Err    error
}

// ReplayKPI replays the actions against the bar prices with the same fill model as the
// simulated execution job (execrunner.SimulateBarWithModel) and builds the instance report.
// Fill quantities are read back through the journal strings, as the SQLite path does.
func ReplayKPI(job KPIJob) KPIResult {
	res := KPIResult{Key: job.Key}
	wallet := paper.Wallet{Coins: make(map[string]float64, len(job.Coins))}
	for c, v := range job.Coins {
		wallet.Coins[c] = v
	}
	position := "flat"
	// held mirrors the sum of the Go pipeline's open units (trackUnits), which a vol_cut is
	// sized from; it differs from the wallet base by dust left over from earlier exits.
	var held float64
	for _, a := range job.Actions {
		i := sort.Search(len(job.Bars), func(i int) bool { return job.Bars[i].CloseMs >= a.CandleCloseMs })
		if i == len(job.Bars) || job.Bars[i].CloseMs != a.CandleCloseMs {
			res.Err = fmt.Errorf("no bar price at candle close %d", a.CandleCloseMs)
			return res
		}
		size := a.Size
		if a.Sizing != nil {
			planned, err := planReplay(&wallet, job, a, job.Bars[i].Close, held)
			if err != nil {
				res.Err = err
				return res
			}
			if planned.Decision == "no-trade" {
				continue
			}
			size = *planned.Size
		}
		_, baseBefore := wallet.QuoteAndBase(instBase(job.Symbol))
		step, err := execrunner.SimulateBarWithModel(&wallet, job.Playbook, job.Symbol, a.CandleCloseMs, a.Decision, &size, job.Bars[i].Close, position, job.Costs, true)
		if err != nil {
			res.Err = err
			return res
		}
		position = step.Position
		_, baseAfter := wallet.QuoteAndBase(instBase(job.Symbol))
		held = trackHeld(held, a, job.Policy.MinOrderQuote, baseBefore, baseAfter, job.Bars[i].Close)
		for _, j := range step.Journals {
			base, _ := strconv.ParseFloat(strings.TrimSpace(j.BaseQty), 64)
			quote, _ := strconv.ParseFloat(strings.TrimSpace(j.QuoteQty), 64)
			price, _ := strconv.ParseFloat(strings.TrimSpace(j.Price), 64)
			res.Fills = append(res.Fills, StrategyKPIFill{
				CandleCloseMs: a.CandleCloseMs, Side: strings.ToLower(j.Side),
				BaseQty: base, QuoteQty: quote, Price: price,
			})
		}
	}
	res.Report = framework.BuildInstanceReportFromSim(job.Run, job.Playbook, job.Symbol, job.FromMs, job.ToMs, job.StartCash, res.Fills, job.Bars)
	return res
}

// planReplay sizes a Sizing action against the replayed wallet. Rescaling needs the C++ equity.
func planReplay(wallet *paper.Wallet, job KPIJob, a ReplayAction, price, held float64) (marketrunner.PlannedAction, error) {
	inst, ok := instruments.Resolve(job.Symbol)
	if !ok {
		return marketrunner.PlannedAction{}, fmt.Errorf("unknown symbol %s", job.Symbol)
	}
	quote, base := wallet.QuoteAndBase(inst.Quote, inst.Base)
	qn, bq := a.Sizing.QuoteNotional, a.Sizing.BaseQty
	if qn != nil || bq != nil {
		if a.Sizing.Equity <= 0 {
			return marketrunner.PlannedAction{}, fmt.Errorf("sized action at %d has equity %v, cannot rescale", a.CandleCloseMs, a.Sizing.Equity)
		}
		k := (quote + base*price) / a.Sizing.Equity
		if h := a.Sizing.HeatBasePerEquity; h != nil && bq != nil {
			cut := held - (quote+base*price)**h
			if !(cut > 0) {
				return marketrunner.PlannedAction{Decision: "no-trade"}, nil
			}
			bq = &cut
			qn = nil
			k = 1
		}
		if qn != nil {
			v := *qn * k
			qn = &v
		}
		if bq != nil {
			v := *bq * k
			bq = &v
		}
	}
	return marketrunner.PlanActionWithPolicy(job.Playbook, job.Symbol, a.Decision, "", quote, base, price, qn, bq, job.Policy, false)
}

// RunKPIWorkers runs a pool of workers over batches of jobs. Each input batch yields one output
// batch of results in the same order, so the consumer (the persistence stage) receives work in
// batches. out is closed after in is closed and all batches are done.
func RunKPIWorkers(workers int, in <-chan []KPIJob, out chan<- []KPIResult) {
	if workers < 1 {
		workers = 1
	}
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for batch := range in {
				results := make([]KPIResult, len(batch))
				for i, job := range batch {
					results[i] = ReplayKPI(job)
				}
				out <- results
			}
		}()
	}
	wg.Wait()
	close(out)
}

func instBase(symbol string) (string, string) {
	inst, _ := instruments.Resolve(symbol)
	return inst.Quote, inst.Base
}

// trackHeld is trackUnits for the total held base: a buy adds the wallet delta, a vol_cut scales
// by the kept share, a full exit or a remainder under the venue minimum clears it.
func trackHeld(held float64, a ReplayAction, minOrderQuote, baseBefore, baseAfter, price float64) float64 {
	switch a.Decision {
	case "buy":
		if baseAfter > baseBefore {
			held += baseAfter - baseBefore
		}
	case "sell":
		partial := a.Sizing != nil && a.Sizing.HeatBasePerEquity != nil
		if !partial || baseBefore <= 0 || baseAfter <= 0 || baseAfter*price < minOrderQuote {
			return 0
		}
		held *= baseAfter / baseBefore
	}
	return held
}
