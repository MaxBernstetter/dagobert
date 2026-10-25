package job

import (
	"os"
	"strconv"

	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/bars"
)

// v1 default until a deployment config validator owns this value.
const defaultAggregationMS int64 = 10_800_000

// AggregationSpec builds rollup parameters from raw interval and job configuration.
func AggregationSpec(sourceIntervalMS int64) bars.AggregationSpec {
	return bars.AggregationSpec{
		SourceIntervalMS: sourceIntervalMS,
		DurationMS:       aggregationDurationMS(),
	}
}

func aggregationDurationMS() int64 {
	raw := os.Getenv("AGGREGATION_DURATION_MS")
	if raw == "" {
		return defaultAggregationMS
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		return defaultAggregationMS
	}
	return v
}
