package kpi

import (
	"encoding/json"

	"github.com/MaxBernstetter/dagobert/src/internal/canonical"
)

// MovingAverage reads an SMA or EMA for the given time_frame_hr from KPI results.
func MovingAverage(results []Result, method string, timeFrameHr int) (float64, bool) {
	def := canonical.IndicatorDef{
		Type:        "MovingAverage",
		Method:      method,
		TimeFrameHr: timeFrameHr,
	}
	key, err := def.String()
	if err != nil {
		return 0, false
	}
	for _, r := range results {
		gotKey, ok := cachedDefKey(r.Def)
		if !ok || gotKey != key {
			continue
		}
		return jsonNumberToFloat(r.Value)
	}
	return 0, false
}

// MovingAverageIndexed is MovingAverage against a prebuilt feature index.
func MovingAverageIndexed(index map[string]float64, method string, timeFrameHr int) (float64, bool) {
	return Feature(index, "MovingAverage", method, timeFrameHr)
}

func jsonNumberToFloat(raw json.RawMessage) (float64, bool) {
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f, true
	}
	var i int64
	if err := json.Unmarshal(raw, &i); err == nil {
		return float64(i), true
	}
	return 0, false
}
