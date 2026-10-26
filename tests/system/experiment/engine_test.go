package experiment_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MaxBernstetter/dagobert/src/backtest"
	"github.com/MaxBernstetter/dagobert/src/backtest/store"
)

// cppSmaCrossBin returns the C++ strategy executable (the worker image sets CPP_SMA_CROSS_BIN).
func cppSmaCrossBin(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("CPP_SMA_CROSS_BIN"); p != "" {
		return p
	}
	for _, preset := range []string{"optimized", "debug"} {
		p, _ := filepath.Abs(filepath.Join("..", "..", "..", "build", preset, "strategies", "sma-cross", "strategy-sma-cross"))
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("strategy-sma-cross not built (task cpp:build) and CPP_SMA_CROSS_BIN unset")
	return ""
}

func TestExperimentEngineCppPersistsResults(t *testing.T) {
	const n = 12
	dataDir, cat, rec := gridExperiment(t, n)
	t.Setenv("EXPERIMENT_ENGINE", "cpp")
	t.Setenv("CPP_SMA_CROSS_BIN", cppSmaCrossBin(t))
	t.Setenv("EXPERIMENT_CPP_SHARD_MAX", "5") // three shards
	if err := backtest.RunExperiment(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}
	got, err := cat.GetExperiment(rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != store.StatusCompleted || countStatus(got)[store.StatusCompleted] != n {
		t.Fatalf("status %q error %q counts %v", got.Status, got.Error, countStatus(got))
	}
	traded := 0
	for _, inst := range got.Instances {
		if inst.StartedAt == nil || inst.FinishedAt == nil {
			t.Fatalf("instance %s missing timing", inst.Label)
		}
		b, err := os.ReadFile(filepath.Join(store.ExperimentWorkspaceDir(dataDir, inst), "report.json"))
		if err != nil {
			t.Fatalf("instance %s report.json: %v", inst.Label, err)
		}
		var rep struct {
			KPIs struct {
				Comparison struct {
					StrategyReturn float64 `json:"strategyReturn"`
				} `json:"comparison"`
			} `json:"kpis"`
		}
		if err := json.Unmarshal(b, &rep); err != nil {
			t.Fatal(err)
		}
		if rep.KPIs.Comparison.StrategyReturn != 0 { // a flat book would return exactly 0
			traded++
		}
	}
	if traded == 0 {
		t.Fatal("no instance traded (strategy return 0 everywhere) on the crossing fixture")
	}
}

func TestExperimentEngineCppCrashedShardFailsItsInstances(t *testing.T) {
	const n = 4
	dataDir, cat, rec := gridExperiment(t, n)
	t.Setenv("EXPERIMENT_ENGINE", "cpp")
	t.Setenv("CPP_SMA_CROSS_BIN", falseBin(t))
	if err := backtest.RunExperiment(dataDir, rec.ID); err == nil {
		t.Fatal("expected the run to fail when every shard crashes")
	}
	got, _ := cat.GetExperiment(rec.ID)
	if got.Status != store.StatusFailed || countStatus(got)[store.StatusFailed] != n {
		t.Fatalf("status %q counts %v", got.Status, countStatus(got))
	}
}

func TestExperimentEngineCppRejectsUnsupportedPlaybook(t *testing.T) {
	dataDir := tempDataDir(t)
	seedLiveMarker(t, dataDir)
	endMs := 240 * hour
	writeCrossingParquet(t, dataDir, symbol, endMs)
	cat, err := store.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer cat.Close()
	settings := store.DefaultExperimentSettings()
	settings.Base, settings.Quote = "BTC", "USDT"
	settings.FromMs, settings.ToMs = 0, endMs
	settings.RedownloadMissing = false
	cfg, _ := json.Marshal(map[string]any{"rsiBull": 52, "rsiBear": 48})
	rec, err := cat.CreateStudy("consensus", "", settings, []store.ExperimentInstanceSpec{
		{Label: "consensus", Playbook: "signal-consensus", StrategyConfig: cfg},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("VENUE_SIMULATE", "1")
	t.Setenv("EXPERIMENT_ENGINE", "cpp")
	t.Setenv("CPP_SMA_CROSS_BIN", falseBin(t))
	if err := backtest.RunExperiment(dataDir, rec.ID); err == nil {
		t.Fatal("expected failure")
	}
	got, _ := cat.GetExperiment(rec.ID)
	if got.Instances[0].Status != store.StatusFailed || !strings.Contains(got.Instances[0].Error, "does not support playbook") {
		t.Fatalf("instance status %q error %q", got.Instances[0].Status, got.Instances[0].Error)
	}
}

// The default engine stays Go: the C++ binary is never touched, so a broken path is harmless.
func TestExperimentDefaultEngineIsGo(t *testing.T) {
	const n = 4
	dataDir, cat, rec := gridExperiment(t, n)
	t.Setenv("CPP_SMA_CROSS_BIN", "/nonexistent")
	if err := backtest.RunExperiment(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := cat.GetExperiment(rec.ID)
	if got.Status != store.StatusCompleted || countStatus(got)[store.StatusCompleted] != n {
		t.Fatalf("status %q counts %v", got.Status, countStatus(got))
	}
}

func TestExperimentEngineInvalidValueFails(t *testing.T) {
	dataDir, _, rec := gridExperiment(t, 2)
	t.Setenv("EXPERIMENT_ENGINE", "rust")
	if err := backtest.RunExperiment(dataDir, rec.ID); err == nil || !strings.Contains(err.Error(), "EXPERIMENT_ENGINE") {
		t.Fatalf("err %v", err)
	}
}

func falseBin(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("false")
	if err != nil {
		t.Skip("no false binary")
	}
	return p
}
