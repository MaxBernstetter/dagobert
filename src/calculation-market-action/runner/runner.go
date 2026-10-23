package runner

import (
	"github.com/MaxBernstetter/trading-copilot/src/calculation-market-action/internal/job"
)

// Run sizes one stored signal into an action (same as the job binary).
func Run(args []string) (exitCode int, detail int) {
	return job.Run(args)
}
