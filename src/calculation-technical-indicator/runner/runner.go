package runner

import (
	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/job"
)

// Run backfills or updates KPIs (same as the job binary).
func Run(args []string, dataDir string, nowMS int64, nowInvalid bool) (detail int, code int) {
	return job.Run(args, dataDir, nowMS, nowInvalid)
}
