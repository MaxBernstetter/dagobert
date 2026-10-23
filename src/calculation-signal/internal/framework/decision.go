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
	Exit     *Exit
	RuleID   string
}
