package run

import (
	"github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/indicators/atr"
	closedistance "github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/indicators/close-distance"
	exponentialmovingaverage "github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/indicators/exponential-moving-average"
	"github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/indicators/macd"
	"github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/indicators"
	"github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/indicators/rsi"
	simplemovingaverage "github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/indicators/simple-moving-average"
	volumemean "github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/indicators/volume-mean"
)

// Deployments lists every indicator series written in algorithm v1.
func Deployments() []indicators.Deployment {
	sma := simplemovingaverage.Indicator{}
	ema := exponentialmovingaverage.Indicator{}
	dist := closedistance.Indicator{}
	macdInd := macd.Indicator{}
	atrInd := atr.Indicator{}
	rsiInd := rsi.Indicator{}
	vol := volumemean.Indicator{}
	return []indicators.Deployment{
		{Key: "sma-3h", TimeFrameHr: 3, Indicator: sma},
		{Key: "sma-6h", TimeFrameHr: 6, Indicator: sma},
		{Key: "sma-12h", TimeFrameHr: 12, Indicator: sma},
		{Key: "sma-24h", TimeFrameHr: 24, Indicator: sma},
		{Key: "sma-48h", TimeFrameHr: 48, Indicator: sma},
		{Key: "ema-3h", TimeFrameHr: 3, Indicator: ema},
		{Key: "close-distance-sma-3h", TimeFrameHr: 3, Indicator: dist},
		{Key: "macd-histogram-3h", TimeFrameHr: 3, Indicator: macdInd},
		{Key: "atr-12h", TimeFrameHr: 12, Indicator: atrInd},
		{Key: "atr-24h", TimeFrameHr: 24, Indicator: atrInd},
		{Key: "rsi-3h", TimeFrameHr: 3, Indicator: rsiInd},
		{Key: "rsi-6h", TimeFrameHr: 6, Indicator: rsiInd},
		{Key: "volume-mean-3h", TimeFrameHr: 3, Indicator: vol},
		{Key: "volume-mean-6h", TimeFrameHr: 6, Indicator: vol},
		{Key: "volume-mean-12h", TimeFrameHr: 12, Indicator: vol},
	}
}
