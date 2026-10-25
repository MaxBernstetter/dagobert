package backtest

// RunExperiment executes prepare → simulate (all instances) → finalize (EXPERIMENT_PHASE=all).
func RunExperiment(dataDir, experimentID string) error {
	if err := RunExperimentPrepare(dataDir, experimentID); err != nil {
		return err
	}
	ids, err := ReadExperimentInstanceList(dataDir, experimentID)
	if err != nil {
		return err
	}
	if err := RunExperimentSimulate(dataDir, experimentID, ids); err != nil {
		return err
	}
	return RunExperimentFinalize(dataDir, experimentID)
}
