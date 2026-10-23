package bars

const msPerHour = 3_600_000

// DurationHr converts a window length in milliseconds to whole hours.
func DurationHr(durationMS int64) int {
	if durationMS <= 0 || durationMS%msPerHour != 0 {
		return 0
	}
	return int(durationMS / msPerHour)
}

// AggregationsForTimeFrame converts a lookback in hours into a number of aggregations.
func AggregationsForTimeFrame(timeFrameHr int, aggregationDurationMS int64) int {
	aggHr := DurationHr(aggregationDurationMS)
	if timeFrameHr <= 0 || aggHr <= 0 || timeFrameHr%aggHr != 0 {
		return 0
	}
	return timeFrameHr / aggHr
}

// MeanLastCloses averages closes over the last n aggregations for the given window.
func MeanLastCloses(series []CandleAggregation, timeFrameHr int) (float64, bool) {
	if len(series) == 0 {
		return 0, false
	}
	n := AggregationsForTimeFrame(timeFrameHr, series[len(series)-1].DurationMS)
	if n <= 0 || len(series) < n {
		return 0, false
	}
	sum := 0.0
	for _, agg := range series[len(series)-n:] {
		sum += agg.Close
	}
	return sum / float64(n), true
}

// MeanLastVolumes averages volume over the last n aggregations for the given window.
func MeanLastVolumes(series []CandleAggregation, timeFrameHr int) (float64, bool) {
	if len(series) == 0 {
		return 0, false
	}
	n := AggregationsForTimeFrame(timeFrameHr, series[len(series)-1].DurationMS)
	if n <= 0 || len(series) < n {
		return 0, false
	}
	sum := 0.0
	for _, agg := range series[len(series)-n:] {
		sum += agg.Volume
	}
	return sum / float64(n), true
}
