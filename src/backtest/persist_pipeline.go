package backtest

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	signalrunner "github.com/MaxBernstetter/dagobert/src/calculation-signal/runner"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/pgcatalog"
	"github.com/MaxBernstetter/dagobert/src/internal/instanceparquet"
	"github.com/google/uuid"
)

// Persistence stage of the experiment simulation. Compute workers produce instanceResult values
// and never touch a database; one persister goroutine batches them and commits each batch in a
// single transaction. An instance is completed in the catalog only once its batch is committed.
// Each flush writes one shard (see instanceparquet) holding the rows of all its instances.

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
	report   signalrunner.InstanceReport
	fills    []signalrunner.StrategyKPIFill
	orders   []instanceparquet.Order
	journals []instanceparquet.Journal
	timing   store.InstanceRunTiming
	// shardKey is the data-dir-relative shard directory holding the instance's rows; set by flush.
	shardKey string
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
		it.ShardKey = r.shardKey
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
	dataDir      string
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
func runPersister(cat *experimentRunCatalog, dataDir, experimentID string, total, alreadyDone int, in <-chan instanceResult) {
	p := &persister{cat: cat, dataDir: dataDir, experimentID: experimentID, total: total, done: alreadyDone}
	parallel := persistParallelPG
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

// flush writes one shard directory with the rows of every instance of the batch (the reports are
// built and uploaded by a pool of 8), then commits the batch with retry and bisection. A failed
// shard write fails every instance of the batch and removes the shard directory; an instance failed
// by its report upload keeps its rows in the shard but gets no shard_key, so nothing reads them.
func (p *persister) flush(batch []instanceResult) {
	data := make([]instanceparquet.InstanceData, len(batch))
	var wg sync.WaitGroup
	jobs := make(chan int)
	for w := 0; w < persistReportWriter; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				r := &batch[i]
				body, err := signalrunner.MarshalInstanceReport(r.report)
				if err == nil {
					err = uploadInstanceReport(r.inst.WorkspaceRelPath, body)
				}
				if err != nil {
					r.err = err
					continue
				}
				data[i] = instanceFiles(*r, body)
			}
		}()
	}
	for i := range batch {
		if batch[i].err == nil {
			jobs <- i
		}
	}
	close(jobs)
	wg.Wait()

	shardKey := fmt.Sprintf("experiments/%s/shards/%s", p.experimentID, uuid.NewString())
	var ok []int
	var rows []instanceparquet.InstanceData
	for i := range batch {
		if batch[i].err == nil {
			ok = append(ok, i)
			rows = append(rows, data[i])
		}
	}
	if len(ok) > 0 {
		dir := filepath.Join(p.dataDir, shardKey)
		if err := instanceparquet.WriteShard(dir, p.experimentID, rows); err != nil {
			if rerr := os.RemoveAll(dir); rerr != nil {
				log.Printf("experiment %s: remove failed shard %s: %v", p.experimentID, shardKey, rerr)
			}
			for _, i := range ok {
				batch[i].err = err
			}
		} else {
			for _, i := range ok {
				batch[i].shardKey = shardKey
			}
		}
	}
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
