package macd

import (
	"github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/bars"
	"github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/indicators"
)

const (
	fastHr   = 12
	slowHr   = 24
	signalHr = 9
)

// Definition is the KPI JSON shape for MACD histogram.
type Definition struct {
	Type              string `json:"type"`
	Method            string `json:"method"`
	FastTimeFrameHr   int    `json:"fast_time_frame_hr"`
	SlowTimeFrameHr   int    `json:"slow_time_frame_hr"`
	SignalTimeFrameHr int    `json:"signal_time_frame_hr"`
}

// Indicator computes MACD histogram on aggregation closes.
type Indicator struct{}

var _ indicators.TechnicalIndicator = Indicator{}

func (Indicator) Slug() string { return "macd" }

func (Indicator) Unit() string { return indicators.UnitPrice }

func (Indicator) Definition(_ int) any {
	return Definition{
		Type:              "Macd",
		Method:            "histogram",
		FastTimeFrameHr:   fastHr,
		SlowTimeFrameHr:   slowHr,
		SignalTimeFrameHr: signalHr,
	}
}

func (Indicator) Calculate(series []bars.CandleAggregation, _ int) (float64, bool) {
	if len(series) == 0 {
		return 0, false
	}
	dur := series[len(series)-1].DurationMS
	slowN := bars.AggregationsForTimeFrame(slowHr, dur)
	sigN := bars.AggregationsForTimeFrame(signalHr, dur)
	if slowN <= 0 || sigN <= 0 || len(series) < slowN+sigN {
		return 0, false
	}
	closes := make([]float64, len(series))
	for i, agg := range series {
		closes[i] = agg.Close
	}
	fastN := bars.AggregationsForTimeFrame(fastHr, dur)
	if fastN <= 0 {
		return 0, false
	}
	macdLine := make([]float64, len(closes))
	for i := slowN - 1; i < len(closes); i++ {
		fast := emaCloses(closes[:i+1], fastN)
		slow := emaCloses(closes[:i+1], slowN)
		macdLine[i] = fast - slow
	}
	last := len(closes) - 1
	signal := emaCloses(macdLine[slowN-1:last+1], sigN)
	hist := macdLine[last] - signal
	return hist, true
}

func emaCloses(closes []float64, period int) float64 {
	if len(closes) == 0 || period <= 0 {
		return 0
	}
	if period > len(closes) {
		period = len(closes)
	}
	window := closes[len(closes)-period:]
	if len(window) == 1 {
		return window[0]
	}
	alpha := 2 / float64(period+1)
	out := window[0]
	for i := 1; i < len(window); i++ {
		out = alpha*window[i] + (1-alpha)*out
	}
	return out
}
