package runner

import (
	"runtime"
	"testing"
	"time"

	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
)

func TestResourceSamplerProducesOptionalStats(t *testing.T) {
	s := startResourceSampler(50 * time.Millisecond)
	// Busy-wait so CPU% deltas are non-zero and enough ticks accumulate.
	deadline := time.Now().Add(350 * time.Millisecond)
	for time.Now().Before(deadline) {
		sum := 0
		for i := 0; i < 10_000; i++ {
			sum += i
		}
		runtime.KeepAlive(sum)
		time.Sleep(5 * time.Millisecond)
	}
	s.Stop()

	var report framework.RunReport
	s.Apply(&report)
	if report.HeapAllocBytes == nil {
		t.Fatal("expected heapAllocBytes stats")
	}
	if report.HeapAllocBytes.Avg <= 0 || report.HeapAllocBytes.Med <= 0 || report.HeapAllocBytes.P95 <= 0 {
		t.Fatalf("heap stats %+v", report.HeapAllocBytes)
	}
	// RSS/CPU need /proc (Linux); skip hard fail on other platforms.
	if report.RSSBytes != nil && report.RSSBytes.Avg <= 0 {
		t.Fatalf("rss stats %+v", report.RSSBytes)
	}
	if report.CPUPercent != nil && report.CPUPercent.Avg < 0 {
		t.Fatalf("cpu stats %+v", report.CPUPercent)
	}
}

func TestStatsFromSamplesBelowMin(t *testing.T) {
	s := startResourceSampler(time.Hour)
	s.Stop()
	var report framework.RunReport
	s.Apply(&report)
	// One sample at start + one at stop → below MinMetricSamples (3) for cpu (needs deltas).
	if report.CPUPercent != nil {
		t.Fatalf("expected nil cpu with too few deltas, got %+v", report.CPUPercent)
	}
}
