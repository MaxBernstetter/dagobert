package store

// InstanceBatchItem is the final state of one instance for a batched catalog write.
// A non-empty Error marks the instance failed; otherwise it is completed with Metrics.
type InstanceBatchItem struct {
	InstanceID string
	Error      string
	Timing     InstanceRunTiming
	Metrics    InstanceListMetrics
}

// PersistInstanceBatch writes status, timing and list metrics of many instances plus the
// experiment progress counter in one transaction.
func (c *Catalog) PersistInstanceBatch(experimentID string, items []InstanceBatchItem, progressDone, progressTotal int) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	failed, err := tx.Prepare(
		`UPDATE experiment_instance SET status = ?, error = ?, started_at = ?, finished_at = ?, run_report_json = '' WHERE id = ?`)
	if err != nil {
		return err
	}
	defer failed.Close()
	done, err := tx.Prepare(
		`UPDATE experiment_instance SET status = ?, error = '', started_at = ?, finished_at = ?, run_report_json = '',
		  metric_duration_ms = ?, metric_cagr = ?, metric_max_drawdown = ?, metric_sharpe = ?, metric_sortino = ?,
		  metric_monthly_return_avg = ?, metric_monthly_return_med = ?, metric_yearly_return_avg = ?,
		  metric_yearly_return_med = ?, metric_trades_per_month_avg = ?, metric_trades_per_month_med = ?,
		  metric_wins = ?, metric_losses = ?, metric_breakeven = ?, metric_win_rate = ?,
		  metric_monthly_win_rate_avg = ?, metric_monthly_win_rate_med = ?,
		  metric_yearly_win_rate_avg = ?, metric_yearly_win_rate_med = ?
		 WHERE id = ?`)
	if err != nil {
		return err
	}
	defer done.Close()
	for _, it := range items {
		t := it.Timing
		if it.Error != "" {
			if _, err := failed.Exec(StatusFailed, it.Error, nullInt64(t.StartedAtMs), nullInt64(t.FinishedAtMs), it.InstanceID); err != nil {
				return err
			}
			continue
		}
		m := it.Metrics
		if _, err := done.Exec(StatusCompleted, nullInt64(t.StartedAtMs), nullInt64(t.FinishedAtMs),
			nullInt(m.DurationMs), nullFloat(m.Cagr), nullFloat(m.MaxDrawdown), nullFloat(m.Sharpe), nullFloat(m.Sortino),
			nullFloat(m.MonthlyReturnAvg), nullFloat(m.MonthlyReturnMed), nullFloat(m.YearlyReturnAvg),
			nullFloat(m.YearlyReturnMed), nullFloat(m.TradesPerMonthAvg), nullFloat(m.TradesPerMonthMed),
			nullInt32(m.Wins), nullInt32(m.Losses), nullInt32(m.Breakeven), nullFloat(m.WinRate),
			nullFloat(m.MonthlyWinRateAvg), nullFloat(m.MonthlyWinRateMed),
			nullFloat(m.YearlyWinRateAvg), nullFloat(m.YearlyWinRateMed),
			it.InstanceID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(
		`UPDATE experiment SET progress_stage = ?, progress_stages = ?, progress_stage_label = ?,
		 progress_instance_done = ?, progress_instance_total = ? WHERE id = ?`,
		4, ExperimentStageTotal, ExperimentStageLabelSimulation, progressDone, progressTotal, experimentID,
	); err != nil {
		return err
	}
	return tx.Commit()
}
