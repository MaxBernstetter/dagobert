package indicators

import (
	"encoding/json"
	"sort"

	"github.com/MaxBernstetter/trading-copilot/services/process/internal/bars"
)

const (
	unitPrice  = "price"
	unitAmount = "amount"
	unitRatio  = "ratio"
	unitIndex  = "index"

	shortHours = 3
	longHours  = 6
)

// Window is the number of trailing bars Results reads.
func Window() int {
	return longHours / 3
}

// Result is one graph node stored on the KPI row.
type Result struct {
	Def   json.RawMessage `json:"def"`
	Value float64         `json:"value"`
	Unit  string          `json:"unit"`
}

// Results builds the requested graph for series, which ends at the bar
// being processed. A node is omitted when its inputs are not available.
func Results(series []bars.Bar) ([]Result, error) {
	if len(series) == 0 {
		return nil, nil
	}
	closes := make([]float64, len(series))
	volumes := make([]float64, len(series))
	for i, bar := range series {
		closes[i] = bar.Close
		volumes[i] = bar.Volume
	}

	shortN := shortHours / 3
	longN := longHours / 3
	closeShort, closeLong, hasLongClose := sharedWindowSums(closes, shortN, longN)
	volumeShort, volumeLong, hasLongVolume := sharedWindowSums(volumes, shortN, longN)

	current := series[len(series)-1]
	sma3 := closeShort / float64(shortN)

	out := make([]Result, 0, 7)
	out = append(out, Result{
		Def:   movingAverageDef("sma", shortHours),
		Value: sma3,
		Unit:  unitPrice,
	})
	if hasLongClose {
		out = append(out, Result{
			Def:   movingAverageDef("sma", longHours),
			Value: closeLong / float64(longN),
			Unit:  unitPrice,
		})
	}
	out = append(out, Result{
		Def:   movingAverageDef("ema", shortHours),
		Value: ema(current.Close, shortN),
		Unit:  unitPrice,
	})
	if sma3 != 0 {
		out = append(out, Result{
			Def:   closeDistanceDef("sma", shortHours),
			Value: (current.Close - sma3) / sma3,
			Unit:  unitRatio,
		})
	}
	out = append(out, Result{
		Def:   rsiDef(shortHours),
		Value: rsi(current.HourClose[0], current.HourClose[1], current.HourClose[2]),
		Unit:  unitIndex,
	})
	out = append(out, Result{
		Def:   volumeMeanDef(shortHours),
		Value: volumeShort / float64(shortN),
		Unit:  unitAmount,
	})
	if hasLongVolume {
		out = append(out, Result{
			Def:   volumeMeanDef(longHours),
			Value: volumeLong / float64(longN),
			Unit:  unitAmount,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return string(out[i].Def) < string(out[j].Def)
	})
	return out, nil
}

// sharedWindowSums sums the last shortN values once. The long window is that
// sum plus only the older values that sit outside the short window, so the
// overlapping bars are not walked again.
func sharedWindowSums(series []float64, shortN, longN int) (shortSum, longSum float64, hasLong bool) {
	end := len(series)
	if shortN <= 0 || end < shortN {
		return 0, 0, false
	}
	shortStart := end - shortN
	for _, value := range series[shortStart:end] {
		shortSum += value
	}
	if longN <= shortN || end < longN {
		return shortSum, 0, false
	}
	longSum = shortSum
	for _, value := range series[end-longN : shortStart] {
		longSum += value
	}
	return shortSum, longSum, true
}

// ema with period 1 has alpha 2/(1+1) = 1, so the value is the current close.
func ema(close float64, period int) float64 {
	if period == 1 {
		return close
	}
	alpha := 2 / float64(period+1)
	return alpha*close + (1-alpha)*close
}

func rsi(c0, c1, c2 float64) float64 {
	gain, loss := meanGainLoss(c1-c0, c2-c1)
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

func movingAverageDef(method string, hours int) json.RawMessage {
	return canonical(map[string]any{
		"type":          "MovingAverage",
		"method":        method,
		"time_frame_hr": hours,
	})
}

func closeDistanceDef(method string, hours int) json.RawMessage {
	return canonical(map[string]any{
		"type":          "CloseDistance",
		"method":        method,
		"time_frame_hr": hours,
	})
}

func rsiDef(hours int) json.RawMessage {
	return canonical(map[string]any{
		"type":          "Rsi",
		"time_frame_hr": hours,
	})
}

func volumeMeanDef(hours int) json.RawMessage {
	return canonical(map[string]any{
		"type":          "VolumeMean",
		"time_frame_hr": hours,
	})
}

func canonical(fields map[string]any) json.RawMessage {
	encoded, err := json.Marshal(fields)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}
