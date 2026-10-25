package framework

// Exit describes how to leave a trade (carried on the action body for downstream stages).
type Exit struct {
	Kind string `json:"kind"`
	Bars int    `json:"bars"`
}

// Decision is the outcome of one strategy invocation.
type Decision struct {
	Decision string
	Size     *float64
	// Optional risk-sized amounts (position-sizing.md). Nil keeps the legacy target weight / full exit.
	QuoteNotional *float64
	BaseQty       *float64
	StopPrice     *float64
	// RequestedRiskPct is unitRiskPct; CappedRiskPct is the risk left after the weight cap.
	RequestedRiskPct *float64
	CappedRiskPct    *float64
	// RaiseStopsBy lifts every open unit's stop by this price step (a pyramid buy).
	RaiseStopsBy *float64
	Exit         *Exit
	RuleID       string
}
