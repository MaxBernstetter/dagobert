package backtest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExperimentPhaseDefault(t *testing.T) {
	t.Setenv("EXPERIMENT_PHASE", "")
	if ExperimentPhase() != "all" {
		t.Fatalf("got %q", ExperimentPhase())
	}
	t.Setenv("EXPERIMENT_PHASE", "prepare")
	if ExperimentPhase() != "prepare" {
		t.Fatalf("got %q", ExperimentPhase())
	}
	t.Setenv("EXPERIMENT_PHASE", "bogus")
	if ExperimentPhase() != "all" {
		t.Fatalf("unknown should default to all, got %q", ExperimentPhase())
	}
}

func TestResolveExperimentInstanceIDsEnv(t *testing.T) {
	t.Setenv("EXPERIMENT_INSTANCE_FILE", "")
	t.Setenv("EXPERIMENT_INSTANCE_IDS", " id1 ,id2 ")
	ids, err := ResolveExperimentInstanceIDs()
	if err != nil || len(ids) != 2 || ids[0] != "id1" {
		t.Fatalf("ids=%v err=%v", ids, err)
	}
}

func TestResolveExperimentInstanceIDsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ids.json")
	if err := os.WriteFile(path, []byte(`["x","y"]`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EXPERIMENT_INSTANCE_IDS", "")
	t.Setenv("EXPERIMENT_INSTANCE_FILE", path)
	ids, err := ResolveExperimentInstanceIDs()
	if err != nil || len(ids) != 2 {
		t.Fatalf("ids=%v err=%v", ids, err)
	}
}
