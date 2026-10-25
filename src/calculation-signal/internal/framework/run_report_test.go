package framework

import (
	"testing"
	"time"
)

func TestNewRunReport(t *testing.T) {
	start := time.UnixMilli(1_000)
	end := time.UnixMilli(1_250)
	r := NewRunReport(start, end)
	if r.StartedAtMs != 1000 || r.FinishedAtMs != 1250 || r.DurationMs != 250 {
		t.Fatalf("got %+v", r)
	}
}

func TestStatsFromSamplesMin(t *testing.T) {
	if StatsFromSamples([]float64{1, 2}) != nil {
		t.Fatal("expected nil below MinMetricSamples")
	}
	s := StatsFromSamples([]float64{1, 2, 3, 4, 5})
	if s == nil {
		t.Fatal("expected stats")
	}
	if s.Avg != 3 {
		t.Fatalf("avg %v", s.Avg)
	}
	if s.Med != 3 {
		t.Fatalf("med %v", s.Med)
	}
}
