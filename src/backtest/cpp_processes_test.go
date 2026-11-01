package backtest

import (
	"runtime"
	"testing"
)

func TestCppProcessesPrecedence(t *testing.T) {
	t.Setenv("EXPERIMENT_CPP_PROCESSES", "5")
	if got := cppProcesses(3); got != 3 {
		t.Fatalf("the experiment's setting wins over the env: %d want 3", got)
	}
	if got := cppProcesses(0); got != 5 {
		t.Fatalf("auto uses the env fallback: %d want 5", got)
	}
	t.Setenv("EXPERIMENT_CPP_PROCESSES", "")
	if got := cppProcesses(0); got != runtime.NumCPU() {
		t.Fatalf("auto with no env uses one per CPU: %d want %d", got, runtime.NumCPU())
	}
	t.Setenv("EXPERIMENT_CPP_PROCESSES", "nope")
	if got := cppProcesses(0); got != runtime.NumCPU() {
		t.Fatalf("an invalid env falls back to one per CPU: %d", got)
	}
}
