package framework

import (
	"math"
	"sort"
	"time"
)

// MinMetricSamples is the minimum sample count required before optional
// resource MetricStats are populated on a RunReport.
const MinMetricSamples = 3

// MetricStats is a distribution over in-run samples (avg / median / p95).
type MetricStats struct {
	Avg float64 `json:"avg"`
	Med float64 `json:"med"`
	P95 float64 `json:"p95"`
}

// RunReport is wall-clock timing for one strategy instance sim, plus optional
// process resource gauges collected by the runner.
type RunReport struct {
	StartedAtMs  int64 `json:"startedAtMs"`
	FinishedAtMs int64 `json:"finishedAtMs"`
	DurationMs   int64 `json:"durationMs"`

	CPUPercent     *MetricStats `json:"cpuPercent,omitempty"`
	RSSBytes       *MetricStats `json:"rssBytes,omitempty"`
	HeapAllocBytes *MetricStats `json:"heapAllocBytes,omitempty"`
}

// NewRunReport builds required timing fields from wall-clock endpoints.
func NewRunReport(start, end time.Time) RunReport {
	started := start.UnixMilli()
	finished := end.UnixMilli()
	return RunReport{
		StartedAtMs:  started,
		FinishedAtMs: finished,
		DurationMs:   finished - started,
	}
}

// StatsFromSamples returns avg/med/p95, or nil when fewer than MinMetricSamples.
func StatsFromSamples(samples []float64) *MetricStats {
	if len(samples) < MinMetricSamples {
		return nil
	}
	sum := 0.0
	for _, v := range samples {
		sum += v
	}
	sorted := append([]float64(nil), samples...)
	sort.Float64s(sorted)
	return &MetricStats{
		Avg: sum / float64(len(samples)),
		Med: percentileSorted(sorted, 0.50),
		P95: percentileSorted(sorted, 0.95),
	}
}

func percentileSorted(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	idx := p * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	w := idx - float64(lo)
	return sorted[lo]*(1-w) + sorted[hi]*w
}
