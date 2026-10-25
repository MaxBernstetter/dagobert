package rsi

import (
	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/bars"
	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/indicators"
)

// Definition is the KPI JSON shape for RSI.
type Definition struct {
	Type        string `json:"type"`
	TimeFrameHr int    `json:"time_frame_hr"`
}

// Indicator is relative strength index.
type Indicator struct{}

var _ indicators.TechnicalIndicator = Indicator{}

func (Indicator) Slug() string { return "rsi" }

func (Indicator) Unit() string { return indicators.UnitIndex }

func (Indicator) Definition(timeFrameHr int) any {
	return Definition{Type: "Rsi", TimeFrameHr: timeFrameHr}
}

func (Indicator) Calculate(series []bars.CandleAggregation, timeFrameHr int) (float64, bool) {
	if len(series) == 0 {
		return 0, false
	}
	if bars.AggregationsForTimeFrame(timeFrameHr, series[len(series)-1].DurationMS) <= 0 {
		return 0, false
	}
	seg := series[len(series)-1].SegmentClose
	if len(seg) < 2 {
		return 0, false
	}
	return rsiFromSegmentCloses(seg), true
}

func rsiFromSegmentCloses(closes []float64) float64 {
	deltas := make([]float64, len(closes)-1)
	for i := 1; i < len(closes); i++ {
		deltas[i-1] = closes[i] - closes[i-1]
	}
	gain, loss := meanGainLoss(deltas...)
	if gain == 0 && loss == 0 {
		return 50
	}
	if loss == 0 && gain > 0 {
		return 100
	}
	return 100 - 100/(1+gain/loss)
}

func meanGainLoss(deltas ...float64) (gain, loss float64) {
	for _, delta := range deltas {
		if delta > 0 {
			gain += delta
		} else if delta < 0 {
			loss += -delta
		}
	}
	n := float64(len(deltas))
	return gain / n, loss / n
}
