package store

// BacktestStageTotal is the fixed number of user-visible run phases.
const BacktestStageTotal = 4

const (
	StageLabelPrepare    = "Preparing"
	StageLabelMarketData = "Market data"
	StageLabelIndicators = "Indicators"
	StageLabelSimulation = "Strategy simulation"
)

// SetRunProgress updates stage metadata and optional unit progress within the current stage
// (bars during simulation; 0/1 for short phases).
func (c *Catalog) SetRunProgress(id string, stage int, label string, unitDone, unitTotal int) error {
	if stage < 1 {
		stage = 1
	}
	if stage > BacktestStageTotal {
		stage = BacktestStageTotal
	}
	if label == "" {
		label = StageLabelPrepare
	}
	_, err := c.db.Exec(
		`UPDATE backtest SET progress_stage = ?, progress_stages = ?, progress_stage_label = ?,
		 progress_bars_done = ?, progress_bars_total = ? WHERE id = ?`,
		stage, BacktestStageTotal, label, unitDone, unitTotal, id,
	)
	return err
}
