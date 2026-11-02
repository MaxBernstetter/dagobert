package experiment_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	skipOrFailInCI(t, "strategy-sma-cross not built (task cpp:build) and CPP_SMA_CROSS_BIN unset")
	return ""
}

// cppSignalConsensusBin returns the C++ consensus executable (the worker image sets CPP_SIGNAL_CONSENSUS_BIN).
func cppSignalConsensusBin(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("CPP_SIGNAL_CONSENSUS_BIN"); p != "" {
		return p
	}
	for _, preset := range []string{"optimized", "debug"} {
		p, _ := filepath.Abs(filepath.Join("..", "..", "..", "build", preset, "strategies", "consensus", "strategy-signal-consensus"))
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	skipOrFailInCI(t, "strategy-signal-consensus not built (task cpp:build) and CPP_SIGNAL_CONSENSUS_BIN unset")
	return ""
}

func TestExperimentPersistsResults(t *testing.T) {
	const n = 12
	dataDir, url, rec := gridExperiment(t, n)
	t.Setenv("CPP_SMA_CROSS_BIN", cppSmaCrossBin(t))
	t.Setenv("EXPERIMENT_CPP_SHARD_MAX", "5") // three shards
	if err := backtest.RunExperiment(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}
	got := loadExperimentPG(t, url, rec.ID)
	if got.Status != store.StatusCompleted || countStatus(got)[store.StatusCompleted] != n {
		t.Fatalf("status %q error %q counts %v", got.Status, got.Error, countStatus(got))
	}
	traded := 0
	for _, inst := range got.Instances {
		if inst.StartedAt == nil || inst.FinishedAt == nil {
			t.Fatalf("instance %s missing timing", inst.Label)
		}
		b, err := os.ReadFile(filepath.Join(instanceDir(dataDir, inst), "report.json"))
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

func TestExperimentCrashedShardFailsItsInstances(t *testing.T) {
	const n = 4
	dataDir, url, rec := gridExperiment(t, n)
	t.Setenv("CPP_SMA_CROSS_BIN", falseBin(t))
	if err := backtest.RunExperiment(dataDir, rec.ID); err == nil {
		t.Fatal("expected the run to fail when every shard crashes")
	}
	got := loadExperimentPG(t, url, rec.ID)
	if got.Status != store.StatusFailed || countStatus(got)[store.StatusFailed] != n {
		t.Fatalf("status %q counts %v", got.Status, countStatus(got))
	}
}

// An explicit cooldown of 0 reaches the C++ engine as 0 (proto optional presence, #128) and runs.
func TestExperimentRunsConsensusZeroCooldown(t *testing.T) {
	bin := cppSignalConsensusBin(t)
	dataDir := tempDataDir(t)
	seedLiveMarker(t, dataDir)
	endMs := 240 * hour
	writeCrossingParquet(t, dataDir, symbol, endMs)
	url := pgScratchDB(t)
	settings := store.DefaultExperimentSettings()
	settings.Base, settings.Quote = "BTC", "USDT"
	settings.FromMs, settings.ToMs = 0, endMs
	settings.RedownloadMissing = false
	cfg, _ := json.Marshal(map[string]any{"minBarsFlatAfterSell": 0, "minBarsLongBeforeSell": 0})
	rec, err := createExperimentPG(url, "zero-cooldown", settings, []fixtureSpec{
		{Label: "zero-cooldown", Playbook: "signal-consensus", Config: cfg},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("DATABASE_URL", url)
	t.Setenv("VENUE_SIMULATE", "1")
	t.Setenv("CPP_SIGNAL_CONSENSUS_BIN", bin)
	if err := backtest.RunExperiment(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}
	got := loadExperimentPG(t, url, rec.ID)
	if got.Instances[0].Status != store.StatusCompleted {
		t.Fatalf("instance status %q error %q", got.Instances[0].Status, got.Instances[0].Error)
	}
}

func TestExperimentSignalConsensusPersistsResults(t *testing.T) {
	bin := cppSignalConsensusBin(t)
	dataDir := tempDataDir(t)
	seedLiveMarker(t, dataDir)
	endMs := 240 * hour
	writeCrossingParquet(t, dataDir, symbol, endMs)
	url := pgScratchDB(t)
	settings := store.DefaultExperimentSettings()
	settings.Base, settings.Quote = "BTC", "USDT"
	settings.FromMs, settings.ToMs = 0, endMs
	settings.RedownloadMissing = false
	var specs []fixtureSpec
	for i, rsi := range []float64{50, 52, 55} {
		cfg, _ := json.Marshal(map[string]any{"rsiBull": rsi})
		specs = append(specs, fixtureSpec{Label: fmt.Sprintf("c%d", i), Playbook: "signal-consensus", Config: cfg})
	}
	rec, err := createExperimentPG(url, "consensus", settings, specs)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("DATABASE_URL", url)
	t.Setenv("VENUE_SIMULATE", "1")
	t.Setenv("CPP_SIGNAL_CONSENSUS_BIN", bin)
	if err := backtest.RunExperiment(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}
	got := loadExperimentPG(t, url, rec.ID)
	if got.Status != store.StatusCompleted || countStatus(got)[store.StatusCompleted] != len(specs) {
		t.Fatalf("status %q error %q counts %v", got.Status, got.Error, countStatus(got))
	}
	for _, inst := range got.Instances {
		if _, err := os.Stat(filepath.Join(instanceDir(dataDir, inst), "report.json")); err != nil {
			t.Fatalf("instance %s: %v", inst.Label, err)
		}
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

// skipOrFailInCI skips locally when a prerequisite is missing, but fails when CI is set, so a broken
// build step cannot turn the system tests into a silent pass.
func skipOrFailInCI(t *testing.T, reason string) {
	t.Helper()
	if os.Getenv("CI") != "" {
		t.Fatal(reason)
	}
	t.Skip(reason)
}
