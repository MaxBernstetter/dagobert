package store

import (
	"database/sql"
	"strings"
)

// InstanceListMetrics are denormalized scalars for experiment instance table sort/filter.
// Nil pointers mean "not yet available" (SQL NULL).
type InstanceListMetrics struct {
	DurationMs        *int64
	Cagr              *float64
	MaxDrawdown       *float64
	Sharpe            *float64
	Sortino           *float64
	MonthlyReturnAvg  *float64
	MonthlyReturnMed  *float64
	YearlyReturnAvg   *float64
	YearlyReturnMed   *float64
	TradesPerMonthAvg *float64
	TradesPerMonthMed *float64
	Wins              *int32
	Losses            *int32
	Breakeven         *int32
	WinRate           *float64
	MonthlyWinRateAvg *float64
	MonthlyWinRateMed *float64
	YearlyWinRateAvg  *float64
	YearlyWinRateMed  *float64
}

func nullFloat(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullInt32(p *int32) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullInt(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

// UpdateExperimentInstanceMetrics writes list-projection columns for one instance.
func (c *Catalog) UpdateExperimentInstanceMetrics(instanceID string, m InstanceListMetrics) error {
	_, err := c.db.Exec(
		`UPDATE experiment_instance SET
		  metric_duration_ms = ?,
		  metric_cagr = ?,
		  metric_max_drawdown = ?,
		  metric_sharpe = ?,
		  metric_sortino = ?,
		  metric_monthly_return_avg = ?,
		  metric_monthly_return_med = ?,
		  metric_yearly_return_avg = ?,
		  metric_yearly_return_med = ?,
		  metric_trades_per_month_avg = ?,
		  metric_trades_per_month_med = ?,
		  metric_wins = ?,
		  metric_losses = ?,
		  metric_breakeven = ?,
		  metric_win_rate = ?,
		  metric_monthly_win_rate_avg = ?,
		  metric_monthly_win_rate_med = ?,
		  metric_yearly_win_rate_avg = ?,
		  metric_yearly_win_rate_med = ?
		 WHERE id = ?`,
		nullInt(m.DurationMs),
		nullFloat(m.Cagr),
		nullFloat(m.MaxDrawdown),
		nullFloat(m.Sharpe),
		nullFloat(m.Sortino),
		nullFloat(m.MonthlyReturnAvg),
		nullFloat(m.MonthlyReturnMed),
		nullFloat(m.YearlyReturnAvg),
		nullFloat(m.YearlyReturnMed),
		nullFloat(m.TradesPerMonthAvg),
		nullFloat(m.TradesPerMonthMed),
		nullInt32(m.Wins),
		nullInt32(m.Losses),
		nullInt32(m.Breakeven),
		nullFloat(m.WinRate),
		nullFloat(m.MonthlyWinRateAvg),
		nullFloat(m.MonthlyWinRateMed),
		nullFloat(m.YearlyWinRateAvg),
		nullFloat(m.YearlyWinRateMed),
		instanceID,
	)
	if err != nil {
		return err
	}
	return nil
}

func migrateInstanceMetricColumns(db *sql.DB) error {
	cols := []string{
		`ALTER TABLE experiment_instance ADD COLUMN metric_duration_ms INTEGER`,
		`ALTER TABLE experiment_instance ADD COLUMN metric_cagr REAL`,
		`ALTER TABLE experiment_instance ADD COLUMN metric_max_drawdown REAL`,
		`ALTER TABLE experiment_instance ADD COLUMN metric_sharpe REAL`,
		`ALTER TABLE experiment_instance ADD COLUMN metric_sortino REAL`,
		`ALTER TABLE experiment_instance ADD COLUMN metric_monthly_return_avg REAL`,
		`ALTER TABLE experiment_instance ADD COLUMN metric_monthly_return_med REAL`,
		`ALTER TABLE experiment_instance ADD COLUMN metric_yearly_return_avg REAL`,
		`ALTER TABLE experiment_instance ADD COLUMN metric_yearly_return_med REAL`,
		`ALTER TABLE experiment_instance ADD COLUMN metric_trades_per_month_avg REAL`,
		`ALTER TABLE experiment_instance ADD COLUMN metric_trades_per_month_med REAL`,
		`ALTER TABLE experiment_instance ADD COLUMN metric_wins INTEGER`,
		`ALTER TABLE experiment_instance ADD COLUMN metric_losses INTEGER`,
		`ALTER TABLE experiment_instance ADD COLUMN metric_breakeven INTEGER`,
		`ALTER TABLE experiment_instance ADD COLUMN metric_win_rate REAL`,
		`ALTER TABLE experiment_instance ADD COLUMN metric_monthly_win_rate_avg REAL`,
		`ALTER TABLE experiment_instance ADD COLUMN metric_monthly_win_rate_med REAL`,
		`ALTER TABLE experiment_instance ADD COLUMN metric_yearly_win_rate_avg REAL`,
		`ALTER TABLE experiment_instance ADD COLUMN metric_yearly_win_rate_med REAL`,
	}
	for _, ddl := range cols {
		if _, err := db.Exec(ddl); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return err
		}
	}
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_experiment_inst_experiment_status ON experiment_instance(experiment_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_experiment_inst_experiment_symbol ON experiment_instance(experiment_id, symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_experiment_inst_experiment_sharpe ON experiment_instance(experiment_id, metric_sharpe)`,
		`CREATE INDEX IF NOT EXISTS idx_experiment_inst_experiment_sortino ON experiment_instance(experiment_id, metric_sortino)`,
		`CREATE INDEX IF NOT EXISTS idx_experiment_inst_experiment_cagr ON experiment_instance(experiment_id, metric_cagr)`,
		`CREATE INDEX IF NOT EXISTS idx_experiment_inst_experiment_maxdd ON experiment_instance(experiment_id, metric_max_drawdown)`,
		`CREATE INDEX IF NOT EXISTS idx_experiment_inst_experiment_duration ON experiment_instance(experiment_id, metric_duration_ms)`,
		`CREATE INDEX IF NOT EXISTS idx_experiment_inst_experiment_mo_ret ON experiment_instance(experiment_id, metric_monthly_return_avg)`,
	}
	for _, ddl := range indexes {
		if _, err := db.Exec(ddl); err != nil {
			return err
		}
	}
	return nil
}
