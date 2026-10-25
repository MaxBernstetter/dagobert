package pgcatalog

import (
	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	"github.com/google/uuid"
)

// UpdateInstanceMetricsParams maps list metrics to sqlc params.
func UpdateInstanceMetricsParams(experimentID, instanceID uuid.UUID, m store.InstanceListMetrics) experimentdb.UpdateInstanceMetricsParams {
	return experimentdb.UpdateInstanceMetricsParams{
		ExperimentID:            experimentID,
		ID:                      instanceID,
		MetricDurationMs:        int8Ptr(m.DurationMs),
		MetricCagr:              float8Ptr(m.Cagr),
		MetricMaxDrawdown:       float8Ptr(m.MaxDrawdown),
		MetricSharpe:            float8Ptr(m.Sharpe),
		MetricSortino:           float8Ptr(m.Sortino),
		MetricMonthlyReturnAvg:  float8Ptr(m.MonthlyReturnAvg),
		MetricMonthlyReturnMed:  float8Ptr(m.MonthlyReturnMed),
		MetricYearlyReturnAvg:   float8Ptr(m.YearlyReturnAvg),
		MetricYearlyReturnMed:   float8Ptr(m.YearlyReturnMed),
		MetricTradesPerMonthAvg: float8Ptr(m.TradesPerMonthAvg),
		MetricTradesPerMonthMed: float8Ptr(m.TradesPerMonthMed),
		MetricWins:              int4Ptr(m.Wins),
		MetricLosses:            int4Ptr(m.Losses),
		MetricBreakeven:         int4Ptr(m.Breakeven),
		MetricMonthlyWinRateAvg: float4Ptr(m.MonthlyWinRateAvg),
		MetricMonthlyWinRateMed: float4Ptr(m.MonthlyWinRateMed),
		MetricYearlyWinRateAvg:  float4Ptr(m.YearlyWinRateAvg),
		MetricYearlyWinRateMed:  float4Ptr(m.YearlyWinRateMed),
	}
}
