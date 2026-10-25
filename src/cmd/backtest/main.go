package main

import (
	"fmt"
	"os"

	"github.com/MaxBernstetter/dagobert/src/backtest"
)

func main() {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "/data"
	}
	experimentID := os.Getenv("EXPERIMENT_ID")
	if experimentID == "" && len(os.Args) > 2 && os.Args[1] == "experiment" {
		experimentID = os.Args[2]
	}
	if experimentID != "" {
		var err error
		switch backtest.ExperimentPhase() {
		case "prepare":
			err = backtest.RunExperimentPrepare(dataDir, experimentID)
		case "simulate":
			ids, idErr := backtest.ResolveExperimentInstanceIDs()
			if idErr != nil {
				fmt.Fprintln(os.Stderr, idErr)
				os.Exit(1)
			}
			err = backtest.RunExperimentSimulate(dataDir, experimentID, ids)
		case "finalize":
			err = backtest.RunExperimentFinalize(dataDir, experimentID)
		default:
			err = backtest.RunExperiment(dataDir, experimentID)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	id := os.Getenv("BACKTEST_ID")
	if id == "" && len(os.Args) > 1 {
		id = os.Args[1]
	}
	if id == "" {
		fmt.Fprintln(os.Stderr, "BACKTEST_ID or EXPERIMENT_ID required")
		os.Exit(1)
	}
	if err := backtest.Run(dataDir, id); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
