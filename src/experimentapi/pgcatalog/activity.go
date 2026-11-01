package pgcatalog

// PeriodStatRow is one bucket for experiment_instance_period_stat.
type PeriodStatRow struct {
	Kind       string
	Key        string
	ReturnPct  *float64
	TradeCount *int32
	WinCount   *int32
}
