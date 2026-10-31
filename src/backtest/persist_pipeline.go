package backtest

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	signalrunner "github.com/MaxBernstetter/dagobert/src/calculation-signal/runner"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/pgcatalog"
	"github.com/MaxBernstetter/dagobert/src/internal/instanceparquet"
)

// Persistence stage of the experiment simulation. Compute workers produce instanceResult values
// and never touch a database; one persister goroutine batches them and commits each batch in a
// single transaction. An instance is completed in the catalog only once its batch is committed.

const (
	persistBatchMax     = 200
	persistBatchWait    = 250 * time.Millisecond
	persistQueueMax     = 4096
	persistReportWriter = 8
	persistRetries      = 3
	// persistParallelPG is the number of batches committed at once on Postgres. Each batch is its
	// own transaction over disjoint instances; one backend is CPU bound by index and foreign-key
	// maintenance of the COPY, so separate connections scale. SQLite has a single writer.
	persistParallelPG = 4
)

// Test seams (system tests only). Zero values mean production behaviour.
var (
	// PersistCommitHook runs before each batch commit attempt; a non-nil error fails the attempt.
	PersistCommitHook func(instanceIDs []string) error
	// PersistRetryBackoff overrides the retry delays (default 100ms, 400ms, 1.6s).
	PersistRetryBackoff []time.Duration
	// SimulateComputeDoneHook runs once all compute workers have finished.
	SimulateComputeDoneHook func()
)

// instanceResult is the output of the compute stage for one instance.
type instanceResult struct {
	inst     store.ExperimentInstance
	sym      string
	ws       string
	report   signalrunner.InstanceReport
	fills    []signalrunner.StrategyKPIFill
	orders   []instanceparquet.Order
	journals []instanceparquet.Journal
	timing   store.InstanceRunTiming
	err      error
}

func (r instanceResult) item() store.InstanceBatchItem {
	it := store.InstanceBatchItem{InstanceID: r.inst.ID, Timing: r.timing}
	if r.err != nil {
		it.Error = r.err.Error()
		return it
	}
	it.Metrics = ListMetricsFromReport(r.report)
	return it
}

func (r instanceResult) pgItem() pgcatalog.InstanceBatchItem {
	it := pgcatalog.InstanceBatchItem{InstanceBatchItem: r.item(), Symbol: r.sym}
	if r.err == nil {
		it.Stats = periodStatRows(r.report, r.fills)
	}
	return it
}

func persistBackoff(attempt int) time.Duration {
	if PersistRetryBackoff != nil {
		if attempt < len(PersistRetryBackoff) {
			return PersistRetryBackoff[attempt]
		}
		return 0
	}
	return 100 * time.Millisecond << (2 * attempt) // 100ms, 400ms, 1.6s
}

type persister struct {
	cat          *experimentRunCatalog
	experimentID string
	total        int
	mu           sync.Mutex
	done         int // instances claimed by a commit attempt; only grows on success
}

// claim reserves n instances for a commit attempt and returns the progress value to store.
func (p *persister) claim(n int) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.done += n
	return p.done
}

func (p *persister) release(n int) {
	p.mu.Lock()
	p.done -= n
	p.mu.Unlock()
}

// runPersister drains results until the channel is closed and returns once everything is flushed.
func runPersister(cat *experimentRunCatalog, experimentID string, total, alreadyDone int, in <-chan instanceResult) {
	p := &persister{cat: cat, experimentID: experimentID, total: total, done: alreadyDone}
	parallel := 1
	if cat.usesPostgres() {
		parallel = persistParallelPG
	}
	sem := make(chan struct{}, parallel)
	var inflight sync.WaitGroup
	defer inflight.Wait()
	batch := make([]instanceResult, 0, persistBatchMax)
	var timer <-chan time.Time
	flush := func() {
		if len(batch) > 0 {
			sem <- struct{}{}
			inflight.Add(1)
			go func(b []instanceResult) {
				defer func() { <-sem; inflight.Done() }()
				p.flush(b)
			}(batch)
			batch = make([]instanceResult, 0, persistBatchMax)
		}
		timer = nil
	}
	for {
		select {
		case r, ok := <-in:
			if !ok {
				flush()
				return
			}
			if len(batch) == 0 {
				timer = time.After(persistBatchWait)
			}
			batch = append(batch, r)
			if len(batch) >= persistBatchMax {
				flush()
			}
		case <-timer:
			flush()
		}
	}
}

// flush writes report.json and the Parquet files of each instance (pool of 8), then commits the
// batch with retry and bisection. A failed instance leaves no Parquet files behind.
func (p *persister) flush(batch []instanceResult) {
	var wg sync.WaitGroup
	jobs := make(chan int)
	for w := 0; w < persistReportWriter; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				r := &batch[i]
				if err := signalrunner.WriteInstanceReport(r.ws, r.report); err != nil {
					r.err = err
				} else if err := writeInstanceFiles(p.experimentID, *r); err != nil {
					r.err = err
				} else if err := uploadInstanceReport(r.inst.WorkspaceRelPath, r.ws); err != nil {
					r.err = err
				}
				if r.err != nil {
					removeInstanceFiles(r.ws)
				}
			}
		}()
	}
	for i := range batch {
		if batch[i].err == nil {
			jobs <- i
		} else {
			removeInstanceFiles(batch[i].ws)
		}
	}
	close(jobs)
	wg.Wait()
	p.commit(batch)
}

// commit retries a failing batch, then splits it in half until single instances remain.
func (p *persister) commit(batch []instanceResult) {
	var err error
	for attempt := 0; attempt <= persistRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(persistBackoff(attempt - 1))
		}
		if err = p.try(batch); err == nil {
			return
		}
	}
	if len(batch) > 1 {
		mid := len(batch) / 2
		p.commit(batch[:mid])
		p.commit(batch[mid:])
		return
	}
	r := batch[0]
	r.err = fmt.Errorf("persist: %w", err)
	removeInstanceFiles(r.ws)
	if ferr := p.cat.PersistInstanceBatch(p.experimentID, []instanceResult{r}, p.claim(1), p.total); ferr != nil {
		log.Printf("experiment %s: instance %s could not be marked failed: %v", p.experimentID, r.inst.ID, ferr)
	}
}

func (p *persister) try(batch []instanceResult) error {
	if PersistCommitHook != nil {
		ids := make([]string, len(batch))
		for i, r := range batch {
			ids[i] = r.inst.ID
		}
		if err := PersistCommitHook(ids); err != nil {
			return err
		}
	}
	err := p.cat.PersistInstanceBatch(p.experimentID, batch, p.claim(len(batch)), p.total)
	if err != nil {
		p.release(len(batch))
	}
	return err
}

// removeInstanceFiles drops the Parquet files of a failed instance. An empty workspace (an instance
// that never reached simulation) has none.
func removeInstanceFiles(ws string) {
	if ws == "" {
		return
	}
	if err := instanceparquet.Remove(ws); err != nil {
		log.Printf("experiment instance files in %s: remove: %v", ws, err)
	}
}
