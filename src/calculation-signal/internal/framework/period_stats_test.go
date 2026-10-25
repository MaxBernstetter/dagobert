package framework

import (
	"testing"
	"time"
)

func TestComputeRunStatisticsSeries(t *testing.T) {
	portfolio := []equityPoint{
		{CloseMs: dateUTC(2024, 1, 5), Value: 100},
		{CloseMs: dateUTC(2024, 1, 20), Value: 110},
		{CloseMs: dateUTC(2024, 2, 5), Value: 110},
		{CloseMs: dateUTC(2024, 2, 20), Value: 99},
	}
	fills := []StrategyKPIFill{
		{CandleCloseMs: dateUTC(2024, 1, 10), Side: "sell"},
		{CandleCloseMs: dateUTC(2024, 2, 10), Side: "sell"},
	}
	stats := ComputeRunStatistics(portfolio, fills, dateUTC(2024, 1, 1), dateUTC(2024, 3, 1))
	if len(stats.MonthlyReturns) != 2 {
		t.Fatalf("monthlyReturns %d", len(stats.MonthlyReturns))
	}
	if stats.MonthlyReturn == nil || stats.TradesPerMonth == nil {
		t.Fatal("expected nested avg/med")
	}
	if stats.MonthlyReturns[0].Period != "2024-01" {
		t.Fatalf("period %s", stats.MonthlyReturns[0].Period)
	}
}

func dateUTC(y, m, d int) int64 {
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC).UnixMilli()
}
