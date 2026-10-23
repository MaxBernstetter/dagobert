package runner

import (
	"github.com/MaxBernstetter/trading-copilot/src/ingest/internal/job"
)

// Run ingests klines (same as the job binary).
func Run(args []string, getenv func(string) string) (code, detail int) {
	return job.Run(args, getenv)
}
