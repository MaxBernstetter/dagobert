package volumemean

import (
	"github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/bars"
	"github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/indicators"
)

// Definition is the KPI JSON shape for mean volume.
type Definition struct {
	Type        string `json:"type"`
	TimeFrameHr int    `json:"time_frame_hr"`
}

// Indicator is mean volume over a time window.
type Indicator struct{}

var _ indicators.TechnicalIndicator = Indicator{}

func (Indicator) Slug() string { return "volume-mean" }

func (Indicator) Unit() string { return indicators.UnitAmount }

func (Indicator) Definition(timeFrameHr int) any {
	return Definition{Type: "VolumeMean", TimeFrameHr: timeFrameHr}
}

func (Indicator) Calculate(series []bars.CandleAggregation, timeFrameHr int) (float64, bool) {
	return bars.MeanLastVolumes(series, timeFrameHr)
}
