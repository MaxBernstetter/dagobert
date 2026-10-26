package runner

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
	execrunner "github.com/MaxBernstetter/dagobert/src/execution/runner"
	"github.com/MaxBernstetter/dagobert/src/internal/venue/paper"
	"github.com/MaxBernstetter/dagobert/src/internal/venue/sim"
)

// ReplayAction is one sized order decision of a strategy instance (the C++ sweep outcome).
// Size is the order quantity as the execution venue takes it: quote spend for a buy, base
// amount for a sell. Bars without an action are no-trade and need no entry.
type ReplayAction struct {
	CandleCloseMs int64
	Decision      string // "buy" or "sell"
	Size          float64
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
	Actions   []ReplayAction // ascending CandleCloseMs
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
	for _, a := range job.Actions {
		i := sort.Search(len(job.Bars), func(i int) bool { return job.Bars[i].CloseMs >= a.CandleCloseMs })
		if i == len(job.Bars) || job.Bars[i].CloseMs != a.CandleCloseMs {
			res.Err = fmt.Errorf("no bar price at candle close %d", a.CandleCloseMs)
			return res
		}
		size := a.Size
		step, err := execrunner.SimulateBarWithModel(&wallet, job.Playbook, job.Symbol, a.CandleCloseMs, a.Decision, &size, job.Bars[i].Close, position, job.Costs, true)
		if err != nil {
			res.Err = err
			return res
		}
		position = step.Position
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
