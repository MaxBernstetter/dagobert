package experimentapi

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// ExperimentParallelism is the per-experiment count of C++ sweep processes. 0 (unset) means auto: one per
// CPU of the worker. The server accepts 0..parallelismMax(); anything above is rejected with a message
// that names the maximum.

// parallelismMax is the server's upper bound: EXPERIMENT_MAX_PARALLELISM when set (for an orchestrator
// that sees a different CPU count than the workers), else the CPUs of this process.
func parallelismMax() int {
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv("EXPERIMENT_MAX_PARALLELISM"))); err == nil && v >= 1 {
		return v
	}
	return runtime.NumCPU()
}

// validateParallelism checks a requested process count: an integer from 0 (auto) up to max.
func validateParallelism(requested, max int) error {
	if requested < 0 {
		return fmt.Errorf("parallelism must be 0 (auto) or a positive number of processes, got %d", requested)
	}
	if requested > max {
		return fmt.Errorf("parallelism %d is above the maximum %d (the CPUs of this server)", requested, max)
	}
	return nil
}

// handleExperimentLimits serves GET /api/v1/experiments/limits: the values the create form needs.
func (s *Server) handleExperimentLimits(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]int{"maxParallelism": parallelismMax()})
}
