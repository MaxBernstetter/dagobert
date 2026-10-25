package pgcatalog

import (
	"testing"

	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	"github.com/google/uuid"
)

func TestUpdateInstanceMetricsParamsFromListMetrics(t *testing.T) {
	dur := int64(1200)
	cagr := 0.12
	sharpe := 1.5
	moAvg := 0.01
	m := store.InstanceListMetrics{
		DurationMs:       &dur,
		Cagr:             &cagr,
		Sharpe:           &sharpe,
		MonthlyReturnAvg: &moAvg,
	}
	sid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	iid := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	p := UpdateInstanceMetricsParams(sid, iid, m)
	if !p.MetricDurationMs.Valid || p.MetricDurationMs.Int64 != 1200 {
		t.Fatalf("duration %v", p.MetricDurationMs)
	}
	if !p.MetricCagr.Valid || p.MetricCagr.Float64 != 0.12 {
		t.Fatalf("cagr %v", p.MetricCagr)
	}
	if !p.MetricSharpe.Valid || p.MetricSharpe.Float64 != 1.5 {
		t.Fatalf("sharpe %v", p.MetricSharpe)
	}
	if !p.MetricMonthlyReturnAvg.Valid || p.MetricMonthlyReturnAvg.Float64 != 0.01 {
		t.Fatalf("monthly avg %v", p.MetricMonthlyReturnAvg)
	}
	wins := int32(3)
	losses := int32(1)
	even := int32(0)
	monthAvg := 0.75
	m.Wins = &wins
	m.Losses = &losses
	m.Breakeven = &even
	m.MonthlyWinRateAvg = &monthAvg
	p = UpdateInstanceMetricsParams(sid, iid, m)
	if !p.MetricWins.Valid || p.MetricWins.Int32 != 3 {
		t.Fatalf("wins %v", p.MetricWins)
	}
	if !p.MetricLosses.Valid || p.MetricLosses.Int32 != 1 || !p.MetricBreakeven.Valid {
		t.Fatalf("losses %v breakeven %v", p.MetricLosses, p.MetricBreakeven)
	}
	if !p.MetricMonthlyWinRateAvg.Valid || p.MetricMonthlyWinRateAvg.Float32 != float32(monthAvg) {
		t.Fatalf("month win avg %v", p.MetricMonthlyWinRateAvg)
	}
}
