package backtest

import (
	"os"
	"testing"
)

func TestExperimentSimWorkers(t *testing.T) {
	t.Setenv("EXPERIMENT_SIM_WORKERS", "")
	if got := ExperimentSimWorkers(); got != DefaultExperimentSimWorkers {
		t.Fatalf("empty env: got %d want %d", got, DefaultExperimentSimWorkers)
	}

	t.Setenv("EXPERIMENT_SIM_WORKERS", "15")
	if got := ExperimentSimWorkers(); got != 15 {
		t.Fatalf("15: got %d", got)
	}

	t.Setenv("EXPERIMENT_SIM_WORKERS", "0")
	if got := ExperimentSimWorkers(); got != DefaultExperimentSimWorkers {
		t.Fatalf("0: got %d want default", got)
	}

	t.Setenv("EXPERIMENT_SIM_WORKERS", "-3")
	if got := ExperimentSimWorkers(); got != DefaultExperimentSimWorkers {
		t.Fatalf("negative: got %d want default", got)
	}

	t.Setenv("EXPERIMENT_SIM_WORKERS", "nope")
	if got := ExperimentSimWorkers(); got != DefaultExperimentSimWorkers {
		t.Fatalf("invalid: got %d want default", got)
	}

	_ = os.Unsetenv("EXPERIMENT_SIM_WORKERS")
}
