package signalconsensus

import (
	"github.com/MaxBernstetter/trading-copilot/src/calculation-signal/internal/framework"
)

const logicVersion = "9"

const (
	barHr          = 6
	signalTrendHr  = 12
	regimeTrendHr  = 24
	macroTrendHr   = 48
	atrHr          = 24

	rsiBull      = 52.0
	rsiBear      = 48.0
	volRatioBull = 1.10
	volRatioBear = 0.85

	atrStopMult = 2.5

	minBarsFlatAfterSell  = 8
	minBarsLongBeforeSell = 4
	noCooldown            = 999
)

// Strategy: 6h bars, higher-level trend stack (SMA24 > SMA48), SMA12 cross + MACD entry.
type Strategy struct{}

func (Strategy) Slug() string { return "signal-consensus" }

func (Strategy) LogicVersion() string { return logicVersion }

func (Strategy) Evaluate(ctx framework.Context) (framework.Decision, error) {
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

	momentum := momentumVote(ctx, rsi, okRSI)
	volume := volumeVote(ctx.BarVolume, volMean, okVolMean)

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
	atrStop := okATR && ctx.BarClose < signal-atrStopMult*atr

	if ctx.Position == "flat" && crossUp && higherTrend && macdOK &&
		momentum != voteBear && volume != voteBear &&
		sinceSell >= minBarsFlatAfterSell {
		return framework.Decision{Decision: "buy", RuleID: "consensus_buy"}, nil
	}

	if ctx.Position == "long" && sinceBuy >= minBarsLongBeforeSell {
		if crossDown {
			return framework.Decision{Decision: "sell", RuleID: "consensus_sell"}, nil
		}
		if atrStop {
			return framework.Decision{Decision: "sell", RuleID: "atr_stop"}, nil
		}
	}

	return noTrade("none"), nil
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

func momentumVote(ctx framework.Context, rsi float64, ok bool) vote {
	if !ok {
		return voteNeutral
	}
	prev, hasPrev := ctx.PreviousRSI(barHr)
	if rsi >= rsiBull || (rsi > 50 && hasPrev && rsi > prev) {
		return voteBull
	}
	if rsi <= rsiBear || (rsi < 50 && hasPrev && rsi < prev) {
		return voteBear
	}
	return voteNeutral
}

func volumeVote(barVol, meanVol float64, okMean bool) vote {
	if !okMean || barVol <= 0 || meanVol <= 0 {
		return voteNeutral
	}
	ratio := barVol / meanVol
	if ratio >= volRatioBull {
		return voteBull
	}
	if ratio <= volRatioBear {
		return voteBear
	}
	return voteNeutral
}

func noTrade(rule string) framework.Decision {
	return framework.Decision{Decision: "no-trade", RuleID: rule}
}

var _ framework.Strategy = Strategy{}
