package bars

// Candle is one closed kline from the lake.
type Candle struct {
	OpenTime  int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	CloseTime int64
}

// AggregationSpec defines how raw candles are rolled up. It is supplied by the job
// from raw-data metadata and deployment configuration, not from package constants.
type AggregationSpec struct {
	SourceIntervalMS int64
	DurationMS       int64
}

// CandleAggregation is one complete aggregated bar built from consecutive source candles.
type CandleAggregation struct {
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	OpenTime  int64
	CloseTime int64
	// DurationMS is the aggregation window length for this row.
	DurationMS int64
	// SegmentClose holds each source candle close in time order within this aggregation.
	SegmentClose []float64
}

func sourceCandlesPerAggregation(spec AggregationSpec) int {
	if spec.SourceIntervalMS <= 0 || spec.DurationMS <= 0 {
		return 0
	}
	if spec.DurationMS%spec.SourceIntervalMS != 0 {
		return 0
	}
	return int(spec.DurationMS / spec.SourceIntervalMS)
}

// Series returns every complete aggregation at a UTC boundary with open time
// at or before aggOpen, ordered by open time. The requested aggregation must be
// complete. Earlier incomplete groups are omitted.
func Series(spec AggregationSpec, candles []Candle, aggOpen int64) ([]CandleAggregation, bool) {
	duration := spec.DurationMS
	if duration <= 0 {
		return nil, false
	}
	byOpen := make(map[int64]Candle, len(candles))
	for _, candle := range candles {
		byOpen[candle.OpenTime] = candle
	}

	opens := make([]int64, 0)
	seen := make(map[int64]struct{})
	consider := func(open int64) {
		if open%duration != 0 || open > aggOpen {
			return
		}
		if _, ok := seen[open]; ok {
			return
		}
		seen[open] = struct{}{}
		opens = append(opens, open)
	}
	consider(aggOpen)
	for open := range byOpen {
		consider(open)
	}
	sortInt64(opens)

	series := make([]CandleAggregation, 0, len(opens))
	for _, open := range opens {
		agg, ok := build(spec, byOpen, open)
		if !ok {
			if open == aggOpen {
				return nil, false
			}
			continue
		}
		series = append(series, agg)
	}
	if len(series) == 0 || series[len(series)-1].OpenTime != aggOpen {
		return nil, false
	}
	return series, true
}

// Ordered returns every complete aggregation in the candles, oldest first.
func Ordered(spec AggregationSpec, candles []Candle) []CandleAggregation {
	duration := spec.DurationMS
	if duration <= 0 || len(candles) == 0 {
		return nil
	}
	byOpen := make(map[int64]Candle, len(candles))
	minOpen := candles[0].OpenTime
	maxOpen := candles[0].OpenTime
	for _, candle := range candles {
		byOpen[candle.OpenTime] = candle
		if candle.OpenTime < minOpen {
			minOpen = candle.OpenTime
		}
		if candle.OpenTime > maxOpen {
			maxOpen = candle.OpenTime
		}
	}
	start := minOpen - minOpen%duration
	out := make([]CandleAggregation, 0, (maxOpen-start)/duration+1)
	for open := start; open <= maxOpen; open += duration {
		agg, ok := build(spec, byOpen, open)
		if ok {
			out = append(out, agg)
		}
	}
	return out
}

func build(spec AggregationSpec, byOpen map[int64]Candle, open int64) (CandleAggregation, bool) {
	steps := sourceCandlesPerAggregation(spec)
	if steps <= 0 {
		return CandleAggregation{}, false
	}
	group := make([]Candle, steps)
	for i := 0; i < steps; i++ {
		c, ok := byOpen[open+int64(i)*spec.SourceIntervalMS]
		if !ok {
			return CandleAggregation{}, false
		}
		group[i] = c
	}
	highs := make([]float64, steps)
	lows := make([]float64, steps)
	segmentClose := make([]float64, steps)
	var volume float64
	for i, c := range group {
		highs[i] = c.High
		lows[i] = c.Low
		segmentClose[i] = c.Close
		volume += c.Volume
	}
	last := group[steps-1]
	return CandleAggregation{
		Open:         group[0].Open,
		High:         maxFloats(highs),
		Low:          minFloats(lows),
		Close:        last.Close,
		Volume:       volume,
		OpenTime:     open,
		CloseTime:    last.CloseTime,
		DurationMS:   spec.DurationMS,
		SegmentClose: segmentClose,
	}, true
}

func maxFloats(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	m := v[0]
	for i := 1; i < len(v); i++ {
		if v[i] > m {
			m = v[i]
		}
	}
	return m
}

func minFloats(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	m := v[0]
	for i := 1; i < len(v); i++ {
		if v[i] < m {
			m = v[i]
		}
	}
	return m
}

func sortInt64(values []int64) {
	for i := 1; i < len(values); i++ {
		v := values[i]
		j := i
		for j > 0 && values[j-1] > v {
			values[j] = values[j-1]
			j--
		}
		values[j] = v
	}
}
