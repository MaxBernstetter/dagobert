package kpi

import (
	"github.com/MaxBernstetter/trading-copilot/src/internal/canonical"
)

// RSI reads RSI for the given aggregation window (hours).
func RSI(results []Result, timeFrameHr int) (float64, bool) {
	return indicatorFloat(results, "Rsi", timeFrameHr, "")
}

// CloseDistanceSMA reads close distance from SMA for the given window.
func CloseDistanceSMA(results []Result, timeFrameHr int) (float64, bool) {
	return indicatorFloat(results, "CloseDistance", timeFrameHr, "sma")
}

// VolumeMean reads mean volume for the given window.
func VolumeMean(results []Result, timeFrameHr int) (float64, bool) {
	return indicatorFloat(results, "VolumeMean", timeFrameHr, "")
}

// MacdHistogram reads the MACD histogram series for the 3h decision clock.
func MacdHistogram(results []Result) (float64, bool) {
	return indicatorMacdHistogram(results)
}

// ATR reads Wilder ATR for the given window in hours.
func ATR(results []Result, timeFrameHr int) (float64, bool) {
	return indicatorFloat(results, "Atr", timeFrameHr, "wilder")
}

func indicatorMacdHistogram(results []Result) (float64, bool) {
	for _, r := range results {
		got, err := canonical.ParseDefFromJSON(r.Def)
		if err != nil {
			continue
		}
		if got.Type != "Macd" || got.Method != "histogram" {
			continue
		}
		return jsonNumberToFloat(r.Value)
	}
	return 0, false
}

func indicatorFloat(results []Result, typ string, timeFrameHr int, method string) (float64, bool) {
	for _, r := range results {
		got, err := canonical.ParseDefFromJSON(r.Def)
		if err != nil {
			continue
		}
		if got.Type != typ || got.TimeFrameHr != timeFrameHr {
			continue
		}
		if method != "" && got.Method != method {
			continue
		}
		return jsonNumberToFloat(r.Value)
	}
	return 0, false
}
