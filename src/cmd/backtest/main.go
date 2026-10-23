package main

import (
	"fmt"
	"os"

	"github.com/MaxBernstetter/trading-copilot/src/backtest"
)

func main() {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "/data"
	}
	id := os.Getenv("BACKTEST_ID")
	if id == "" && len(os.Args) > 1 {
		id = os.Args[1]
	}
	if id == "" {
		fmt.Fprintln(os.Stderr, "BACKTEST_ID required")
		os.Exit(1)
	}
	if err := backtest.Run(dataDir, id); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
