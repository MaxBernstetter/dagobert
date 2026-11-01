package backtest

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	marketrunner "github.com/MaxBernstetter/dagobert/src/calculation-market-action/runner"
	signalrunner "github.com/MaxBernstetter/dagobert/src/calculation-signal/runner"
	tirunner "github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/runner"
	"github.com/MaxBernstetter/dagobert/src/internal/signalstore"
	"github.com/MaxBernstetter/dagobert/src/internal/venue/paper"
	"github.com/MaxBernstetter/dagobert/src/internal/venue/sim"
	"github.com/MaxBernstetter/dagobert/src/shared"
)

// The experiment simulate stage runs on the C++ engine (issue #130): C++ sweep (actions only) ->
// ReplayKPI (fills, equity, KPIs) -> the shared persistence stage. There is no Go engine and no
// per-instance fallback: an unsupported playbook or a missing binary fails the instance.

const (
	// DefaultCppShardMax caps the instances per C++ process (EXPERIMENT_CPP_SHARD_MAX).
	DefaultCppShardMax = 500
	cppKPIBatch        = 64
)

// cppBinaryEnv names the env var holding the executable of a playbook slug.
func cppBinaryEnv(slug string) (string, error) {
	switch slug {
	case "ma-cross":
		return "CPP_SMA_CROSS_BIN", nil
	case "signal-consensus":
		return "CPP_SIGNAL_CONSENSUS_BIN", nil
	}
	return "", fmt.Errorf("C++ engine does not support playbook %q", slug)
}

func cppBinaryPath(slug string) (string, error) {
	name, err := cppBinaryEnv(slug)
	if err != nil {
		return "", err
	}
	p := strings.TrimSpace(os.Getenv(name))
	if p == "" {
		return "", fmt.Errorf("%s is not set: the experiment engine needs the C++ strategy binary", name)
	}
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return p, nil
}

// cppProcesses is the number of concurrent C++ sweep processes: the experiment's own setting when set
// (validated by the API), else EXPERIMENT_CPP_PROCESSES (the fallback default), else one per CPU.
func cppProcesses(setting int) int {
	if setting >= 1 {
		return setting
	}
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("EXPERIMENT_CPP_PROCESSES"))); err == nil && n >= 1 {
		return n
	}
	return runtime.NumCPU()
}

// cppShardSize is ceil(n/procs), capped at EXPERIMENT_CPP_SHARD_MAX (default 500).
func cppShardSize(n, procs int) int {
	limit := DefaultCppShardMax
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv("EXPERIMENT_CPP_SHARD_MAX"))); err == nil && v >= 1 {
		limit = v
	}
	if procs < 1 {
		procs = 1
	}
	return max(1, min((n+procs-1)/procs, limit))
}

type cppShard struct {
	sym   string
	slug  string
	insts []store.ExperimentInstance
}

// cppSimulate runs the compute stage on the C++ engine and feeds results to queue (not closed).
func cppSimulate(dataDir string, meta experimentPrepareMeta, bySymbol map[string]experimentSymKPI, compute []store.ExperimentInstance, queue chan<- instanceResult) {
	settings := meta.Settings
	aggMS := signalstore.AggregationDurationMS()
	startCash := settings.InitialUSDC + settings.InitialUSDT
	fail := func(inst store.ExperimentInstance, err error) {
		queue <- instanceResult{inst: inst, sym: store.InstanceSymbol(inst, settings), ws: store.ExperimentWorkspaceDir(dataDir, inst), err: err}
	}

	// Group by (symbol, playbook), then shard. Unsupported playbooks fail their instances.
	groups := map[[2]string][]store.ExperimentInstance{}
	var order [][2]string
	for _, inst := range compute {
		if _, err := cppBinaryPath(inst.Playbook); err != nil {
			fail(inst, err)
			continue
		}
		k := [2]string{store.InstanceSymbol(inst, settings), inst.Playbook}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], inst)
	}
	procs := cppProcesses(meta.Settings.Parallelism)
	var shards []cppShard
	for _, k := range order {
		insts := groups[k]
		size := cppShardSize(len(insts), procs)
		for i := 0; i < len(insts); i += size {
			shards = append(shards, cppShard{sym: k[0], slug: k[1], insts: insts[i:min(i+size, len(insts))]})
		}
	}

	ranges := map[string]tirunner.BarRange{}
	rangeErr := map[string]error{}
	for _, sh := range shards {
		if _, ok := ranges[sh.sym]; ok || rangeErr[sh.sym] != nil {
			continue
		}
		br, err := tirunner.BuildBarRange(dataDir, sh.sym, meta.From, meta.To, meta.NowMS)
		if err != nil {
			rangeErr[sh.sym] = fmt.Errorf("bar range: %w", err)
			continue
		}
		ranges[sh.sym] = br
	}

	jobs := make(chan []signalrunner.KPIJob, procs)
	results := make(chan []signalrunner.KPIResult, procs)
	byKey := map[string]store.ExperimentInstance{}
	for _, inst := range compute {
		byKey[inst.ID] = inst
	}
	go signalrunner.RunKPIWorkers(ExperimentSimWorkers(), jobs, results)
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for batch := range results {
			for _, r := range batch {
				inst := byKey[r.Key]
				res := instanceResult{inst: inst, sym: store.InstanceSymbol(inst, settings), ws: store.ExperimentWorkspaceDir(dataDir, inst), report: r.Report, fills: r.Fills, orders: r.Orders, journals: r.Journals, err: r.Err}
				if r.Err == nil {
					res.timing = store.InstanceRunTiming{StartedAtMs: r.Report.Run.StartedAtMs, FinishedAtMs: r.Report.Run.FinishedAtMs}
				}
				queue <- res
			}
		}
	}()

	sem := make(chan struct{}, procs)
	var wg sync.WaitGroup
	for _, sh := range shards {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			failAll := func(err error) {
				for _, inst := range sh.insts {
					fail(inst, err)
				}
			}
			if err := rangeErr[sh.sym]; err != nil {
				failAll(err)
				return
			}
			bin, err := cppBinaryPath(sh.slug)
			if err != nil {
				failAll(err)
				return
			}
			br := ranges[sh.sym]
			items := make([]SweepItemSpec, len(sh.insts))
			for i, inst := range sh.insts {
				items[i] = SweepItemSpec{Key: inst.ID, Playbook: inst.Playbook, StrategyConfig: inst.StrategyConfig}
			}
			base, quote, ok := shared.SplitPipelineSymbol(sh.sym)
			if !ok {
				base, quote = settings.Base, settings.Quote
			}
			req, err := BuildSweepRequest(settings, base, quote, br, startCash, items)
			if err != nil {
				failAll(err)
				return
			}
			start := time.Now()
			reports, err := RunSweep(context.Background(), bin, req)
			if err != nil {
				failAll(err) // a crashed shard fails its instances; no Go fallback
				return
			}
			run := signalrunner.RunReport{StartedAtMs: start.UnixMilli()}
			run.FinishedAtMs = time.Now().UnixMilli()
			run.DurationMs = run.FinishedAtMs - run.StartedAtMs
			var bars []signalrunner.StrategyKPIBar
			for _, b := range br.Bars {
				if b.OpenMS >= br.EvalFromMs {
					bars = append(bars, signalrunner.StrategyKPIBar{CloseMs: b.OpenMS + aggMS, Close: b.Close})
				}
			}
			var batch []signalrunner.KPIJob
			flush := func() {
				if len(batch) > 0 {
					jobs <- batch
					batch = nil
				}
			}
			for i, inst := range sh.insts {
				actions, err := ReplayActions(reports[i], aggMS)
				if err != nil {
					fail(inst, err)
					continue
				}
				wallet, err := paper.LoadWallet(store.ExperimentWorkspaceDir(dataDir, inst))
				if err != nil {
					fail(inst, fmt.Errorf("paper wallet: %w", err))
					continue
				}
				batch = append(batch, signalrunner.KPIJob{
					Key: inst.ID, Playbook: inst.PlaybookKey, Symbol: sh.sym, FromMs: meta.From, ToMs: meta.To,
					StartCash: startCash, Coins: wallet.Coins, Costs: sim.FromEnv(), Policy: marketrunner.PolicyFromEnv(),
					Actions: actions, Bars: bars, Run: run,
				})
				if len(batch) >= cppKPIBatch {
					flush()
				}
			}
			flush()
		}()
	}
	wg.Wait()
	close(jobs)
	<-drained
}
