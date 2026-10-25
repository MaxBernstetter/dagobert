package simplemovingaverage

import (
	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/bars"
	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/indicators"
)

// Definition is the KPI JSON shape for this indicator (MovingAverage, method sma).
type Definition struct {
	Type        string `json:"type"`
	Method      string `json:"method"`
	TimeFrameHr int    `json:"time_frame_hr"`
}

// Indicator is the simple moving average formula.
type Indicator struct{}

var _ indicators.TechnicalIndicator = Indicator{}

func (Indicator) Slug() string { return "simple-moving-average" }

func (Indicator) Unit() string { return indicators.UnitPrice }

func (Indicator) Definition(timeFrameHr int) any {
	return Definition{Type: "MovingAverage", Method: "sma", TimeFrameHr: timeFrameHr}
}

func (Indicator) Calculate(series []bars.CandleAggregation, timeFrameHr int) (float64, bool) {
	return bars.MeanLastCloses(series, timeFrameHr)
}
