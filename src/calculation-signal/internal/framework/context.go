package framework

import (
	"github.com/MaxBernstetter/trading-copilot/src/internal/kpi"
)

// Context is the input sample for one strategy invocation.
type Context struct {
	Symbol      string
	CandleClose int64
	BarOpen     int64
	// BarClose and BarVolume come from the KPI bar body when available.
	BarClose         float64
	BarVolume        float64
	PreviousBarClose float64
	// BarsSinceBuy/BarsSinceSell count 3h bars since the last buy/sell signal in this replay (999 = no cooldown).
	BarsSinceBuy  int
	BarsSinceSell int
	// Position is rebuilt from prior actions (flat or long).
	Position string
	// Results are KPI values for this bar (BarOpen).
	Results []kpi.Result
	// PreviousResults are KPI values for the prior aggregation bar, when present.
	PreviousResults []kpi.Result
	HasPreviousBar  bool
}

// MovingAverage reads an SMA or EMA for the given time_frame_hr on this bar.
func (c Context) MovingAverage(method string, timeFrameHr int) (float64, bool) {
	return kpi.MovingAverage(c.Results, method, timeFrameHr)
}

// PreviousMovingAverage reads an SMA or EMA on the prior bar.
func (c Context) PreviousMovingAverage(method string, timeFrameHr int) (float64, bool) {
	if !c.HasPreviousBar {
		return 0, false
	}
	return kpi.MovingAverage(c.PreviousResults, method, timeFrameHr)
}

func (c Context) RSI(timeFrameHr int) (float64, bool) {
	return kpi.RSI(c.Results, timeFrameHr)
}

func (c Context) PreviousRSI(timeFrameHr int) (float64, bool) {
	if !c.HasPreviousBar {
		return 0, false
	}
	return kpi.RSI(c.PreviousResults, timeFrameHr)
}

func (c Context) CloseDistanceSMA(timeFrameHr int) (float64, bool) {
	return kpi.CloseDistanceSMA(c.Results, timeFrameHr)
}

func (c Context) VolumeMean(timeFrameHr int) (float64, bool) {
	return kpi.VolumeMean(c.Results, timeFrameHr)
}

func (c Context) MacdHistogram() (float64, bool) {
	return kpi.MacdHistogram(c.Results)
}

func (c Context) PreviousMacdHistogram() (float64, bool) {
	if !c.HasPreviousBar {
		return 0, false
	}
	return kpi.MacdHistogram(c.PreviousResults)
}

func (c Context) ATR(timeFrameHr int) (float64, bool) {
	return kpi.ATR(c.Results, timeFrameHr)
}
