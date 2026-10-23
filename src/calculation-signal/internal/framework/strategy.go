package framework

// Strategy evaluates one candle sample from rebuilt context (no persisted state).
type Strategy interface {
	Slug() string
	LogicVersion() string
	Evaluate(ctx Context) (Decision, error)
}
