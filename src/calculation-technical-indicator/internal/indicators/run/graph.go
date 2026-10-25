package run

import (
	"sort"

	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/bars"
	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/indicators"
)

// Result is one graph node stored on the KPI row.
type Result = indicators.Result

// longestTimeFrameHr is the maximum time_frame_hr among v1 deployments.
const longestTimeFrameHr = 48

// Window is the number of trailing aggregations Results may read for the longest deployment.
func Window(aggregationDurationMS int64) int {
	return bars.AggregationsForTimeFrame(longestTimeFrameHr, aggregationDurationMS)
}

// Results builds the indicator graph for series, which ends at the aggregation being processed.
func Results(series []bars.CandleAggregation) ([]Result, error) {
	if len(series) == 0 {
		return nil, nil
	}
	var out []Result
	for _, dep := range Deployments() {
		value, ok := dep.Indicator.Calculate(series, dep.TimeFrameHr)
		if !ok {
			continue
		}
		out = append(out, Result{
			Def:   indicators.MarshalDefinition(dep.Indicator.Definition(dep.TimeFrameHr)),
			Value: value,
			Unit:  dep.Indicator.Unit(),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return string(out[i].Def) < string(out[j].Def)
	})
	return out, nil
}
