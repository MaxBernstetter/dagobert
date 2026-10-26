package pgcatalog

// PeriodStatRow is one bucket for experiment_instance_period_stat.
type PeriodStatRow struct {
	Kind       string
	Key        string
	ReturnPct  *float64
	TradeCount *int32
	WinCount   *int32
}

// FillRow is one fill for experiment_instance_fill.
type FillRow struct {
	TsMs  int64
	Side  string
	Qty   float64
	Price float64
}

// TradeOutcomeRow is one winning or losing sell.
type TradeOutcomeRow struct {
	Side    int16
	Quote   float32
	Percent *float32
}
