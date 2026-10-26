package runner

import (
	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/job"
)

// Bar is one aggregated OHLCV bar (open time to open time + DurationMS).
type Bar struct {
	OpenMS       int64
	Open         float64
	High         float64
	Low          float64
	Close        float64
	Volume       float64
	DurationMS   int64
	SegmentClose []float64 // close of each source candle inside the bar
}

// BarRange is the bar series of an experiment window, with warmup bars before EvalFromMs.
// Bars from EvalFromMs on are the bars the Go experiment path evaluates; bars before it only
// warm up indicators. The Go path cannot trade on its first bar (no previous bar), so
// TradeFromMs is the bar after EvalFromMs.
type BarRange struct {
	Bars        []Bar
	EvalFromMs  int64
	TradeFromMs int64
}

// BuildBarRange returns the aggregated bars the indicator range for [from, to) is built from,
// including the warmup bars before the first evaluated bar (same selection as BuildKPIRange).
func BuildBarRange(dataDir, symbol string, from, to, nowMS int64) (BarRange, error) {
	r, err := job.BuildBarRange(dataDir, symbol, from, to, nowMS)
	if err != nil {
		return BarRange{}, err
	}
	out := BarRange{Bars: make([]Bar, len(r.Bars)), EvalFromMs: r.EvalFromMs, TradeFromMs: r.TradeFromMs}
	for i, b := range r.Bars {
		out.Bars[i] = Bar{
			OpenMS: b.OpenTime, Open: b.Open, High: b.High, Low: b.Low, Close: b.Close,
			Volume: b.Volume, DurationMS: b.DurationMS, SegmentClose: b.SegmentClose,
		}
	}
	return out, nil
}
