package closedistance

import (
	"github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/bars"
	"github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/indicators"
)

// Definition is the KPI JSON shape for close distance from a moving average.
type Definition struct {
	Type        string `json:"type"`
	Method      string `json:"method"`
	TimeFrameHr int    `json:"time_frame_hr"`
}

// Indicator is close distance from a simple moving average of the same window.
type Indicator struct{}

var _ indicators.TechnicalIndicator = Indicator{}

func (Indicator) Slug() string { return "close-distance" }

func (Indicator) Unit() string { return indicators.UnitRatio }

func (Indicator) Definition(timeFrameHr int) any {
	return Definition{Type: "CloseDistance", Method: "sma", TimeFrameHr: timeFrameHr}
}

func (Indicator) Calculate(series []bars.CandleAggregation, timeFrameHr int) (float64, bool) {
	mean, ok := bars.MeanLastCloses(series, timeFrameHr)
	if !ok || mean == 0 || len(series) == 0 {
		return 0, false
	}
	close := series[len(series)-1].Close
	return (close - mean) / mean, true
}
