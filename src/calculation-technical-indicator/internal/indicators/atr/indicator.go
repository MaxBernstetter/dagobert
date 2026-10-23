package atr

import (
	"math"

	"github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/bars"
	"github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/indicators"
)

// Definition is the KPI JSON shape for ATR.
type Definition struct {
	Type        string `json:"type"`
	Method      string `json:"method"`
	TimeFrameHr int    `json:"time_frame_hr"`
}

// Indicator is average true range (Wilder smoothing).
type Indicator struct{}

var _ indicators.TechnicalIndicator = Indicator{}

func (Indicator) Slug() string { return "atr" }

func (Indicator) Unit() string { return indicators.UnitPrice }

func (Indicator) Definition(timeFrameHr int) any {
	return Definition{Type: "Atr", Method: "wilder", TimeFrameHr: timeFrameHr}
}

func (Indicator) Calculate(series []bars.CandleAggregation, timeFrameHr int) (float64, bool) {
	if len(series) < 2 {
		return 0, false
	}
	n := bars.AggregationsForTimeFrame(timeFrameHr, series[len(series)-1].DurationMS)
	if n <= 0 || len(series) < n+1 {
		return 0, false
	}
	trs := make([]float64, 0, len(series)-1)
	for i := 1; i < len(series); i++ {
		prev := series[i-1]
		cur := series[i]
		tr := math.Max(cur.High-cur.Low,
			math.Max(math.Abs(cur.High-prev.Close), math.Abs(cur.Low-prev.Close)))
		trs = append(trs, tr)
	}
	if len(trs) < n {
		return 0, false
	}
	// Wilder: seed with SMA of first n TRs, then smooth.
	start := len(trs) - n
	atr := 0.0
	for i := start; i < start+n; i++ {
		atr += trs[i]
	}
	atr /= float64(n)
	for i := start + n; i < len(trs); i++ {
		atr = (atr*float64(n-1) + trs[i]) / float64(n)
	}
	return atr, true
}
