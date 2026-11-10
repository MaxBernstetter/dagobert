package experimentapi

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
)

// ExperimentParallelism is the per-experiment count of C++ sweep processes. 0 (unset) means auto: one per
// CPU of the worker. The server accepts 0..parallelismMax(); anything above is rejected with a message
// that names the maximum.

// parallelismMax is the server's upper bound: EXPERIMENT_MAX_PARALLELISM when set (for a server
// whose CPU count differs from the workers, e.g. the Lambda), else the CPUs of this process.
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

func (s *Server) GetExperimentLimits(context.Context, *connect.Request[v1.GetExperimentLimitsRequest]) (*connect.Response[v1.GetExperimentLimitsResponse], error) {
	return connect.NewResponse(&v1.GetExperimentLimitsResponse{MaxParallelism: int32(parallelismMax())}), nil
}
