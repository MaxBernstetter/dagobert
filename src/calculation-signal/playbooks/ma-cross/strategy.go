package macross

import (
	"github.com/MaxBernstetter/trading-copilot/src/calculation-signal/internal/framework"
)

const logicVersion = "3"

// Strategy is SMA(3h) crossing SMA(6h), long-only spot. Signals only; sizing is layer 2.
type Strategy struct{}

func (Strategy) Slug() string { return "ma-cross" }

func (Strategy) LogicVersion() string { return logicVersion }

func (Strategy) Evaluate(ctx framework.Context) (framework.Decision, error) {
	fast, okFast := ctx.MovingAverage("sma", 3)
	slow, okSlow := ctx.MovingAverage("sma", 6)
	if !okFast || !okSlow {
		return noTrade("none"), nil
	}

	prevFast, okPF := ctx.PreviousMovingAverage("sma", 3)
	prevSlow, okPS := ctx.PreviousMovingAverage("sma", 6)
	hasPriorBar := ctx.HasPreviousBar && okPF && okPS

	above := fast > slow
	prevAbove := prevFast > prevSlow
	dec := framework.Decision{Decision: "no-trade", RuleID: "none"}

	if hasPriorBar {
		bullish := !prevAbove && above
		bearish := prevAbove && !above
		switch {
		case ctx.Position == "flat" && bullish:
			dec = framework.Decision{Decision: "buy", RuleID: "golden_cross"}
		case ctx.Position == "long" && bearish:
			dec = framework.Decision{Decision: "sell", RuleID: "death_cross"}
		}
	}

	return dec, nil
}

func noTrade(rule string) framework.Decision {
	return framework.Decision{Decision: "no-trade", RuleID: rule}
}

var _ framework.Strategy = Strategy{}
