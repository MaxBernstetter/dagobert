package runner

import (
	"github.com/MaxBernstetter/dagobert/src/execution/internal/job"
)

// Run executes one candle's action (same as the job binary).
func Run(args []string) (exitCode int, detail int) {
	return job.Run(args)
}
