package experiment_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MaxBernstetter/dagobert/src/backtest"
	"github.com/MaxBernstetter/dagobert/src/backtest/store"
)

// pgScratchDB creates an empty database loaded with db/sqlc/schema.sql on the dev Postgres and
// drops it on cleanup. It skips when no Postgres is reachable. POSTGRES_TEST_URL (default: the
// compose Postgres on 127.0.0.1:9432) names a user that may CREATE DATABASE.
// Many batches commit concurrently on Postgres; one poisoned instance goes through the bisection
// path. Counts must match the journals and metrics, and progress must never go backwards.
func TestExperimentPostgresConcurrentBatches(t *testing.T) {
	const n = 300 // batches flush at 200 instances or after 250 ms while compute is still running
	dataDir, url, rec := gridExperiment(t, n)

	poison := rec.Instances[n/2].ID
	var attempts atomic.Int32 // successful batch commits
	backtest.PersistRetryBackoff = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	backtest.PersistCommitHook = func(ids []string) error {
		for _, id := range ids {
			if id == poison {
				return errors.New("injected connection loss")
			}
		}
		attempts.Add(1)
		return nil
	}
	t.Cleanup(func() { backtest.PersistCommitHook, backtest.PersistRetryBackoff = nil, nil })

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var (
		wg       sync.WaitGroup
		stop     = make(chan struct{})
		lastSeen int32
		backward atomic.Bool
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			var done int32
			if err := pool.QueryRow(ctx, `SELECT coalesce(progress_instance_done,0) FROM experiment`).Scan(&done); err == nil {
				if done < lastSeen {
					backward.Store(true)
				}
				lastSeen = done
			}
			select {
			case <-stop:
				return
			case <-time.After(20 * time.Millisecond):
			}
		}
	}()
	err = backtest.RunExperiment(dataDir, rec.ID)
	close(stop)
	wg.Wait()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("successful batch commits: %d", attempts.Load())
	if attempts.Load() < 6 {
		t.Fatalf("only %d successful batch commits, want at least 6", attempts.Load())
	}
	if backward.Load() {
		t.Fatal("progress_instance_done went backwards")
	}
	var done, total int32
	if err := pool.QueryRow(ctx, `SELECT progress_instance_done, progress_instance_total FROM experiment`).Scan(&done, &total); err != nil {
		t.Fatal(err)
	}
	if done != n || total != n {
		t.Fatalf("progress %d/%d want %d/%d", done, total, n, n)
	}

	var completed, failed int
	if err := pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE status=$1), count(*) FILTER (WHERE status=$2) FROM experiment_instance`,
		store.StatusCompleted, store.StatusFailed).Scan(&completed, &failed); err != nil {
		t.Fatal(err)
	}
	if completed != n-1 || failed != 1 {
		t.Fatalf("completed %d failed %d want %d and 1", completed, failed, n-1)
	}
	var poisonStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM experiment_instance WHERE id=$1`, poison).Scan(&poisonStatus); err != nil || poisonStatus != store.StatusFailed {
		t.Fatalf("poisoned instance status %q err %v", poisonStatus, err)
	}

	// Fills and trade outcomes are files now: Postgres keeps none of them, and each instance that
	// completed has its journal, fill and outcome files, the poisoned one has none.
	wantFills, wantOutcomes := 0, 0
	for _, inst := range rec.Instances {
		ws := filepath.Join(dataDir, inst.WorkspaceRelPath)
		if inst.ID == poison {
			for _, name := range []string{"journals.parquet", "fills.parquet", "trade_outcomes.parquet", "orders.parquet"} {
				if _, err := os.Stat(filepath.Join(ws, name)); !os.IsNotExist(err) {
					t.Fatalf("poisoned instance keeps %s: %v", name, err)
				}
			}
			continue
		}
		wantFills += len(readJournalFile(t, ws))
		wantOutcomes += len(readOutcomeFile(t, ws))
	}
	var gotOutcomes, statInstances int
	var fillTable, outcomeTable *string
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.experiment_instance_fill')::text, to_regclass('public.experiment_trade_outcome')::text`).Scan(&fillTable, &outcomeTable); err != nil {
		t.Fatal(err)
	}
	if fillTable != nil || outcomeTable != nil {
		t.Fatalf("dropped tables still exist: fill=%v outcome=%v", fillTable, outcomeTable)
	}
	if err := pool.QueryRow(ctx, `SELECT coalesce(sum(metric_wins+metric_losses+metric_breakeven),0) FROM experiment_instance WHERE status=$1`, store.StatusCompleted).Scan(&gotOutcomes); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(DISTINCT instance_id) FROM experiment_instance_period_stat`).Scan(&statInstances); err != nil {
		t.Fatal(err)
	}
	if wantFills == 0 {
		t.Fatal("no fills were written to the journal files")
	}
	if gotOutcomes != wantOutcomes || wantOutcomes == 0 {
		t.Fatalf("outcomes in metrics %d, outcome files %d", gotOutcomes, wantOutcomes)
	}
	if statInstances != n-1 {
		t.Fatalf("period stats for %d instances want %d", statInstances, n-1)
	}
}
