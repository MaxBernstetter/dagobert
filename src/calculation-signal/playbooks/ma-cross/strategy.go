package macross

import (
	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
)

const logicVersion = "4"

// Strategy is SMA(fast) crossing SMA(slow), long-only spot. Signals only; sizing is layer 2.
// Windows come from Context.StrategyConfig (defaults: 3h/6h).
type Strategy struct{}

func (Strategy) Slug() string { return "ma-cross" }

func (Strategy) LogicVersion() string { return logicVersion }

func (Strategy) Evaluate(ctx framework.Context) (framework.Decision, error) {
	cfg, err := ParseConfig(ctx.StrategyConfig)
	if err != nil {
		return framework.Decision{}, err
	}
	if err := cfg.Validate(framework.AggregationDurationMS()); err != nil {
		return framework.Decision{}, err
	}

	fast, okFast := ctx.MovingAverage("sma", cfg.FastHr)
	slow, okSlow := ctx.MovingAverage("sma", cfg.SlowHr)
	if !okFast || !okSlow {
		return noTrade("none"), nil
	}

	prevFast, okPF := ctx.PreviousMovingAverage("sma", cfg.FastHr)
	prevSlow, okPS := ctx.PreviousMovingAverage("sma", cfg.SlowHr)
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
