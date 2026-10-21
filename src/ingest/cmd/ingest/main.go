package main

import (
	"fmt"
	"os"

	"github.com/MaxBernstetter/trading-copilot/src/ingest/internal/job"
)

func main() {
	code, detail := job.Run(os.Args[1:], os.Getenv)
	fmt.Printf("DETAIL=%d\n", detail)
	os.Exit(composeExit(code))
}

// Compose treats only exit 0 as a finished step. Already-stored work is exit 2.
// COMPOSE_OK_ON_ABORT=1 maps that to 0 so the next service starts. The printed DETAIL stays 1.
func composeExit(code int) int {
	if code == 2 && os.Getenv("COMPOSE_OK_ON_ABORT") == "1" {
		return 0
	}
	return code
}
