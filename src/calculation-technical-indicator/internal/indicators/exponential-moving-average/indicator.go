package exponentialmovingaverage

import (
	"github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/bars"
	"github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/indicators"
)

// Definition is the KPI JSON shape for this indicator (MovingAverage, method ema).
type Definition struct {
	Type        string `json:"type"`
	Method      string `json:"method"`
	TimeFrameHr int    `json:"time_frame_hr"`
}

// Indicator is the exponential moving average formula.
type Indicator struct{}

var _ indicators.TechnicalIndicator = Indicator{}

func (Indicator) Slug() string { return "exponential-moving-average" }

func (Indicator) Unit() string { return indicators.UnitPrice }

func (Indicator) Definition(timeFrameHr int) any {
	return Definition{Type: "MovingAverage", Method: "ema", TimeFrameHr: timeFrameHr}
}

func (Indicator) Calculate(series []bars.CandleAggregation, timeFrameHr int) (float64, bool) {
	if len(series) == 0 {
		return 0, false
	}
	n := bars.AggregationsForTimeFrame(timeFrameHr, series[len(series)-1].DurationMS)
	if n <= 0 || len(series) < n {
		return 0, false
	}
	return ema(series[len(series)-n:], n), true
}

// ema applies EMA recurrence over window (one close per aggregation), α = 2/(period+1).
func ema(window []bars.CandleAggregation, period int) float64 {
	if period == 1 {
		return window[len(window)-1].Close
	}
	alpha := 2 / float64(period+1)
	out := window[0].Close
	for i := 1; i < len(window); i++ {
		close := window[i].Close
		out = alpha*close + (1-alpha)*out
	}
	return out
}
