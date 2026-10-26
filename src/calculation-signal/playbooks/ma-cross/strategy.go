package macross

import (
	"math"

	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/sizing"
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
	bullish := hasPriorBar && !prevAbove && above
	bearish := hasPriorBar && prevAbove && !above

	if cfg.Sizing != nil {
		return evaluateSized(ctx, *cfg.Sizing, bullish, bearish), nil
	}

	dec := framework.Decision{Decision: "no-trade", RuleID: "none"}
	switch {
	case ctx.Position == "flat" && bullish:
		dec = framework.Decision{Decision: "buy", RuleID: "golden_cross"}
	case ctx.Position == "long" && bearish:
		dec = framework.Decision{Decision: "sell", RuleID: "death_cross"}
	}
	return dec, nil
}

// atrWindowHr is the ATR window the stop and the size are measured in.
const atrWindowHr = 24

// evaluateSized applies position-sizing.md. While long, one bar does one thing, in
// order: stop exit, death-cross exit, volatility cut, pyramid buy. Stop, cut and
// pyramid need tracked open units; a long position without them only has the cross exit.
func evaluateSized(ctx framework.Context, sz sizing.Config, bullish, bearish bool) framework.Decision {
	atr, okATR := ctx.ATR(atrWindowHr)
	okATR = okATR && atr > 0
	price := ctx.BarClose

	if ctx.Position != "long" {
		if ctx.Position != "flat" || !bullish || !okATR || price <= 0 {
			return noTrade("none")
		}
		n := sizing.Notional(ctx.Equity, atr, price, sz)
		if n <= 0 || sizing.BelowMin(n, sz) {
			return noTrade("none")
		}
		return sizedBuy(n, price, atr, ctx.Equity, sz, "golden_cross")
	}

	if len(ctx.OpenUnits) > 0 && price > 0 {
		stop := math.Inf(-1)
		for _, u := range ctx.OpenUnits {
			stop = math.Max(stop, u.Stop)
		}
		if price <= stop {
			return framework.Decision{Decision: "sell", RuleID: "atr_stop"}
		}
	}
	if bearish {
		return framework.Decision{Decision: "sell", RuleID: "death_cross"}
	}
	if len(ctx.OpenUnits) == 0 || !okATR || price <= 0 || ctx.Equity <= 0 {
		return noTrade("none")
	}

	var held float64
	for _, u := range ctx.OpenUnits {
		held += u.Qty
	}
	if cut := sizing.ExcessBaseForHeat(held, atr, ctx.Equity, sz); cut > 0 {
		if sizing.BelowMin(cut*price, sz) {
			return noTrade("none")
		}
		return framework.Decision{Decision: "sell", RuleID: "vol_cut", BaseQty: &cut}
	}
	if n, ok := sizing.PyramidAllowed(ctx.OpenUnits, ctx.LastFill, price, ctx.Equity, atr, sz); ok {
		if sizing.BelowMin(n, sz) {
			return noTrade("none")
		}
		dec := sizedBuy(n, price, atr, ctx.Equity, sz, "pyramid")
		step := sizing.RaiseStops(ctx.OpenUnits[:1], atr, sz)[0].Stop - ctx.OpenUnits[0].Stop
		dec.RaiseStopsBy = &step
		return dec
	}
	return noTrade("none")
}

func sizedBuy(notional, price, atr, equity float64, sz sizing.Config, rule string) framework.Decision {
	stop := sizing.StopPrice(price, atr, sz)
	req, capped := sizing.RiskPct(notional, equity, atr, price, sz)
	return framework.Decision{
		Decision: "buy", RuleID: rule,
		QuoteNotional: &notional, StopPrice: &stop,
		RequestedRiskPct: &req, CappedRiskPct: &capped,
	}
}

func noTrade(rule string) framework.Decision {
	return framework.Decision{Decision: "no-trade", RuleID: rule}
}

var _ framework.Strategy = Strategy{}
