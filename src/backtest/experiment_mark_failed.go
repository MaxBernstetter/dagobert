package backtest

// MarkExperimentFailed sets an experiment to failed with message. The workflow calls it when a phase
// crashed before it could mark the experiment itself (Lambda timeout, out of memory).
func MarkExperimentFailed(experimentID, message string) error {
	cat, err := openExperimentRunCatalog()
	if err != nil {
		return err
	}
	defer cat.Close()
	return cat.MarkExperimentFailed(experimentID, message)
}
