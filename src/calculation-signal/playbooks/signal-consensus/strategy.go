package signalconsensus

import (
	"math"

	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/sizing"
)

// logicVersion is unchanged: sizing is opt-in config, and the config JSON is part of the playbook hash.
const logicVersion = "10"

const (
	barHr         = 3
	signalTrendHr = 12
	regimeTrendHr = 24
	macroTrendHr  = 48
	atrHr         = 24

	defaultRsiBull      = 52.0
	defaultRsiBear      = 48.0
	defaultVolRatioBull = 1.10
	defaultVolRatioBear = 0.85
	defaultAtrStopMult  = 2.5

	defaultMinBarsFlatAfterSell  = 8
	defaultMinBarsLongBeforeSell = 4
	noCooldown                   = 999
)

// Strategy: 3h bars, higher-level trend stack (SMA24 > SMA48), SMA12 cross + MACD entry.
type Strategy struct{}

func (Strategy) Slug() string { return "signal-consensus" }

func (Strategy) LogicVersion() string { return logicVersion }

func (Strategy) Evaluate(ctx framework.Context) (framework.Decision, error) {
	cfg, err := ParseConfig(ctx.StrategyConfig)
	if err != nil {
		return framework.Decision{}, err
	}
	if err := cfg.Validate(0); err != nil {
		return framework.Decision{}, err
	}

	signal, okSignal := ctx.MovingAverage("sma", signalTrendHr)
	prevSignal, okPS := ctx.PreviousMovingAverage("sma", signalTrendHr)
	regime, okRegime := ctx.MovingAverage("sma", regimeTrendHr)
	macro, okMacro := ctx.MovingAverage("sma", macroTrendHr)
	rsi, okRSI := ctx.RSI(barHr)
	volMean, okVolMean := ctx.VolumeMean(barHr)
	atr, okATR := ctx.ATR(atrHr)
	hist, okHist := ctx.MacdHistogram()

	if ctx.BarClose <= 0 || !okRSI || !okSignal {
		return noTrade("none"), nil
	}

	sinceBuy := ctx.BarsSinceBuy
	if sinceBuy <= 0 {
		sinceBuy = noCooldown
	}
	sinceSell := ctx.BarsSinceSell
	if sinceSell <= 0 {
		sinceSell = noCooldown
	}

	momentum := momentumVote(ctx, rsi, okRSI, cfg)
	volume := volumeVote(ctx.BarVolume, volMean, okVolMean, cfg)

	hasPrior := ctx.HasPreviousBar && okPS && ctx.PreviousBarClose > 0
	if !hasPrior {
		return noTrade("none"), nil
	}

	crossUp := crossedAbove(ctx.PreviousBarClose, ctx.BarClose, prevSignal, signal)
	crossDown := crossedBelow(ctx.PreviousBarClose, ctx.BarClose, prevSignal, signal)
	higherTrend := higherLevelTrend(ctx.BarClose, regime, macro, okRegime, okMacro)
	macdOK := okHist && hist > 0
	if !okHist {
		macdOK = momentum == voteBull
	}
	atrStop := okATR && ctx.BarClose < signal-cfg.AtrStopMult*atr

	if cfg.Sizing != nil {
		if d, ok := sizedDecision(ctx, cfg, atr, okATR, crossDown, sinceBuy, atrStop, entry(ctx, crossUp, higherTrend, macdOK, momentum, volume, sinceSell, cfg)); ok {
			return d, nil
		}
		if ctx.Position == "flat" || len(ctx.OpenUnits) > 0 {
			return noTrade("none"), nil
		}
	}

	if ctx.Position == "flat" && crossUp && higherTrend && macdOK &&
		momentum != voteBear && volume != voteBear &&
		sinceSell >= cfg.MinBarsFlatAfterSell {
		return framework.Decision{Decision: "buy", RuleID: "consensus_buy"}, nil
	}

	if ctx.Position == "long" && sinceBuy >= cfg.MinBarsLongBeforeSell {
		if crossDown {
			return framework.Decision{Decision: "sell", RuleID: "consensus_sell"}, nil
		}
		if atrStop {
			return framework.Decision{Decision: "sell", RuleID: "atr_stop"}, nil
		}
	}

	return noTrade("none"), nil
}

func entry(ctx framework.Context, crossUp, higherTrend, macdOK bool, momentum, volume vote, sinceSell int, cfg Config) bool {
	return ctx.Position == "flat" && crossUp && higherTrend && macdOK &&
		momentum != voteBear && volume != voteBear && sinceSell >= cfg.MinBarsFlatAfterSell
}

// sizedDecision applies position-sizing.md. ok is false when nothing sized applies and
// the legacy rules should run (long without tracked units). The stop is fill-based and
// stopAtrMult is the playbook's atrStopMult.
func sizedDecision(ctx framework.Context, cfg Config, atr float64, okATR, crossDown bool, sinceBuy int, legacyStop, enter bool) (framework.Decision, bool) {
	sc := *cfg.Sizing
	mult := cfg.AtrStopMult
	sc.StopAtrMult = &mult
	px := ctx.BarClose

	if ctx.Position == "flat" {
		if !enter || !okATR || ctx.Equity <= 0 {
			return framework.Decision{}, false
		}
		n := sizing.Notional(ctx.Equity, atr, px, sc)
		if n <= 0 || sizing.BelowMin(n, sc) {
			return framework.Decision{}, false
		}
		return sizedBuy(n, px, atr, ctx.Equity, sc, "consensus_buy"), true
	}
	if len(ctx.OpenUnits) == 0 {
		return framework.Decision{}, false
	}

	var held, stop float64
	stop = math.Inf(-1)
	for _, u := range ctx.OpenUnits {
		held += u.Qty
		stop = math.Max(stop, u.Stop)
	}
	if px <= stop {
		return framework.Decision{Decision: "sell", RuleID: "atr_stop"}, true
	}
	if sinceBuy >= cfg.MinBarsLongBeforeSell && crossDown {
		return framework.Decision{Decision: "sell", RuleID: "consensus_sell"}, true
	}
	if !okATR {
		return framework.Decision{}, false
	}
	if q := sizing.ExcessBaseForHeat(held, atr, ctx.Equity, sc); q > 0 {
		if sizing.BelowMin(q*px, sc) {
			return framework.Decision{}, false
		}
		return framework.Decision{Decision: "sell", BaseQty: &q, RuleID: "vol_cut"}, true
	}
	if n, ok := sizing.PyramidAllowed(ctx.OpenUnits, ctx.LastFill, px, ctx.Equity, atr, sc); ok {
		if sizing.BelowMin(n, sc) {
			return framework.Decision{}, false
		}
		d := sizedBuy(n, px, atr, ctx.Equity, sc, "pyramid")
		step := *sc.AddAtrMult * atr
		d.RaiseStopsBy = &step
		return d, true
	}
	return framework.Decision{}, false
}

func sizedBuy(n, px, atr, equity float64, sc sizing.Config, rule string) framework.Decision {
	stop := sizing.StopPrice(px, atr, sc)
	req, capped := sizing.RiskPct(n, equity, atr, px, sc)
	return framework.Decision{Decision: "buy", RuleID: rule, QuoteNotional: &n, StopPrice: &stop,
		RequestedRiskPct: &req, CappedRiskPct: &capped}
}

// Bullish higher-timeframe stack: price above 24h/48h SMAs and SMA24 > SMA48.
func higherLevelTrend(close, regime, macro float64, okRegime, okMacro bool) bool {
	if !okRegime || !okMacro || regime <= 0 || macro <= 0 {
		return false
	}
	return close > regime && close > macro && regime > macro
}

type vote int

const (
	voteNeutral vote = 0
	voteBull    vote = 1
	voteBear    vote = -1
)

func crossedAbove(prevClose, close, prevBaseline, baseline float64) bool {
	return prevClose <= prevBaseline && close > baseline
}

func crossedBelow(prevClose, close, prevBaseline, baseline float64) bool {
	return prevClose >= prevBaseline && close < baseline
}

func momentumVote(ctx framework.Context, rsi float64, ok bool, cfg Config) vote {
	if !ok {
		return voteNeutral
	}
	prev, hasPrev := ctx.PreviousRSI(barHr)
	if rsi >= cfg.RsiBull || (rsi > 50 && hasPrev && rsi > prev) {
		return voteBull
	}
	if rsi <= cfg.RsiBear || (rsi < 50 && hasPrev && rsi < prev) {
		return voteBear
	}
	return voteNeutral
}

func volumeVote(barVol, meanVol float64, okMean bool, cfg Config) vote {
	if !okMean || barVol <= 0 || meanVol <= 0 {
		return voteNeutral
	}
	ratio := barVol / meanVol
	if ratio >= cfg.VolRatioBull {
		return voteBull
	}
	if ratio <= cfg.VolRatioBear {
		return voteBear
	}
	return voteNeutral
}

func noTrade(rule string) framework.Decision {
	return framework.Decision{Decision: "no-trade", RuleID: rule}
}

var _ framework.Strategy = Strategy{}
