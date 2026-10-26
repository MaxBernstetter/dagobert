package experiment_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MaxBernstetter/dagobert/src/backtest"
	"github.com/MaxBernstetter/dagobert/src/backtest/store"
)

// gridConfig returns the i-th distinct ma-cross config: 10 valid SMA pairs times stop multiples.
func gridConfig(i int) map[string]any {
	pairs := [][2]int{{3, 6}, {3, 12}, {3, 24}, {3, 48}, {6, 12}, {6, 24}, {6, 48}, {12, 24}, {12, 48}, {24, 48}}
	p := pairs[i%len(pairs)]
	return map[string]any{
		"fastHr": p[0], "slowHr": p[1],
		"sizing": map[string]any{"stopAtrMult": 0.5 + 0.01*float64(i/len(pairs))},
	}
}

// gridExperiment seeds a crossing fixture and an ma-cross grid of n instances.
func gridExperiment(t *testing.T, n int) (string, *store.Catalog, store.ExperimentRecord) {
	t.Helper()
	dataDir := tempDataDir(t)
	seedLiveMarker(t, dataDir)
	endMs := 240 * hour
	if h, _ := strconv.Atoi(os.Getenv("BENCH_HOURS")); h > 0 {
		endMs = int64(h) * hour
	}
	writeCrossingParquet(t, dataDir, symbol, endMs)
	cat, err := store.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cat.Close() })
	settings := store.DefaultExperimentSettings()
	settings.Base, settings.Quote = "BTC", "USDT"
	settings.FromMs, settings.ToMs = 0, endMs
	settings.RedownloadMissing = false
	var specs []store.ExperimentInstanceSpec
	for i := 0; i < n; i++ {
		cfg, _ := json.Marshal(gridConfig(i))
		specs = append(specs, store.ExperimentInstanceSpec{Label: fmt.Sprintf("g%02d", i), Playbook: "ma-cross", StrategyConfig: cfg})
	}
	rec, err := cat.CreateStudy("grid", "", settings, specs)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("VENUE_SIMULATE", "1")
	return dataDir, cat, rec
}

func countStatus(rec store.ExperimentRecord) map[string]int {
	out := map[string]int{}
	for _, inst := range rec.Instances {
		out[inst.Status]++
	}
	return out
}

func TestExperimentPersistsAllInstances(t *testing.T) {
	const n = 20
	dataDir, cat, rec := gridExperiment(t, n)
	if err := backtest.RunExperiment(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}
	got, err := cat.GetExperiment(rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != store.StatusCompleted {
		t.Fatalf("status %q error %q", got.Status, got.Error)
	}
	if c := countStatus(got); c[store.StatusCompleted] != n {
		t.Fatalf("status counts %v want %d completed", c, n)
	}
	if got.ProgressInstanceDone != n || got.ProgressInstanceTotal != n {
		t.Fatalf("progress %d/%d want %d/%d", got.ProgressInstanceDone, got.ProgressInstanceTotal, n, n)
	}
	for _, inst := range got.Instances {
		if inst.StartedAt == nil || inst.FinishedAt == nil {
			t.Fatalf("instance %s missing timing", inst.Label)
		}
		if _, err := os.Stat(filepath.Join(store.ExperimentWorkspaceDir(dataDir, inst), "report.json")); err != nil {
			t.Fatalf("instance %s report.json: %v", inst.Label, err)
		}
	}
}

func TestExperimentPersistenceFailureIsRetryable(t *testing.T) {
	const n = 12
	dataDir, cat, rec := gridExperiment(t, n)
	poison := map[string]bool{rec.Instances[2].ID: true, rec.Instances[7].ID: true}
	backtest.PersistRetryBackoff = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	backtest.PersistCommitHook = func(ids []string) error {
		for _, id := range ids {
			if poison[id] {
				return errors.New("injected connection loss")
			}
		}
		return nil
	}
	t.Cleanup(func() { backtest.PersistCommitHook, backtest.PersistRetryBackoff = nil, nil })

	if err := backtest.RunExperiment(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}
	got, err := cat.GetExperiment(rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != store.StatusPartial {
		t.Fatalf("status %q want partial (error %q)", got.Status, got.Error)
	}
	if c := countStatus(got); c[store.StatusCompleted] != n-2 || c[store.StatusFailed] != 2 {
		t.Fatalf("status counts %v want %d completed, 2 failed", countStatus(got), n-2)
	}
	for _, inst := range got.Instances {
		if poison[inst.ID] != (inst.Status == store.StatusFailed) {
			t.Fatalf("instance %s status %q", inst.ID, inst.Status)
		}
		if poison[inst.ID] && !strings.HasPrefix(inst.Error, "persist:") {
			t.Fatalf("failed instance error %q want persist: prefix", inst.Error)
		}
	}

	backtest.PersistCommitHook = nil
	t.Setenv("EXPERIMENT_RETRY_FAILED", "1")
	if err := backtest.RunExperiment(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}
	got, err = cat.GetExperiment(rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != store.StatusCompleted {
		t.Fatalf("after retry status %q error %q", got.Status, got.Error)
	}
	if c := countStatus(got); c[store.StatusCompleted] != n {
		t.Fatalf("after retry counts %v want %d completed", c, n)
	}
}

// Calculation must finish while the persistence stage is still stuck on a slow database.
func TestExperimentComputeIndependentOfPersistenceLatency(t *testing.T) {
	const n = 20
	dataDir, cat, rec := gridExperiment(t, n)
	const delay = 3 * time.Second
	var computeDoneAt, firstCommitAt atomic.Int64
	backtest.PersistCommitHook = func([]string) error {
		time.Sleep(delay)
		firstCommitAt.CompareAndSwap(0, time.Now().UnixNano())
		return nil
	}
	backtest.SimulateComputeDoneHook = func() { computeDoneAt.Store(time.Now().UnixNano()) }
	t.Cleanup(func() { backtest.PersistCommitHook, backtest.SimulateComputeDoneHook = nil, nil })

	if err := backtest.RunExperiment(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}
	if computeDoneAt.Load() == 0 || firstCommitAt.Load() == 0 {
		t.Fatal("hooks did not run")
	}
	if computeDoneAt.Load() >= firstCommitAt.Load() {
		t.Fatalf("compute finished %v after the first slowed commit; compute waited on persistence",
			time.Duration(computeDoneAt.Load()-firstCommitAt.Load()))
	}
	got, err := cat.GetExperiment(rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != store.StatusCompleted || countStatus(got)[store.StatusCompleted] != n {
		t.Fatalf("status %q counts %v", got.Status, countStatus(got))
	}
}
