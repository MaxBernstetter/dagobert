package main

import (
	"fmt"
	"os"

	"github.com/MaxBernstetter/trading-copilot/services/execution/internal/job"
)

// Invocation: execution <symbol> <candle_close_ms>
// Environment: DATA_DIR, PLAYBOOK; BYBIT_BASE_URL (optional); BYBIT_API_KEY, BYBIT_API_SECRET when calling Bybit.
func main() {
	exitCode, detail := job.Run(os.Args[1:])
	fmt.Printf("DETAIL=%d\n", detail)
	os.Exit(exitCode)
}
