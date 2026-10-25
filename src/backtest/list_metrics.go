package backtest

import (
	"math"

	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	"github.com/MaxBernstetter/dagobert/src/calculation-performance"
	signalrunner "github.com/MaxBernstetter/dagobert/src/calculation-signal/runner"
)

func f64Ptr(v float64) *float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return &v
}

// ListMetricsFromReport projects report.json fields used by the instance table.
func ListMetricsFromReport(report signalrunner.InstanceReport) store.InstanceListMetrics {
	m := store.InstanceListMetrics{}
	if report.Run.DurationMs > 0 {
		d := report.Run.DurationMs
		m.DurationMs = &d
	}
	for _, r := range report.KPIs.Results {
		v := f64Ptr(r.Value)
		switch r.Def.Slug {
		case "cagr":
			m.Cagr = v
		case "max-drawdown":
			m.MaxDrawdown = v
		case "sharpe":
			m.Sharpe = v
		case "sortino":
			m.Sortino = v
		}
	}
	st := report.Statistics
	if st.MonthlyReturn != nil {
		m.MonthlyReturnAvg = f64Ptr(st.MonthlyReturn.Average)
		m.MonthlyReturnMed = f64Ptr(st.MonthlyReturn.Median)
	}
	if st.YearlyReturn != nil {
		m.YearlyReturnAvg = f64Ptr(st.YearlyReturn.Average)
		m.YearlyReturnMed = f64Ptr(st.YearlyReturn.Median)
	}
	if st.TradesPerMonth != nil {
		m.TradesPerMonthAvg = f64Ptr(st.TradesPerMonth.Average)
		m.TradesPerMonthMed = f64Ptr(st.TradesPerMonth.Median)
	}
	applyPerformanceMetrics(&m, report.Performance)
	return m
}

func applyPerformanceMetrics(m *store.InstanceListMetrics, perf performance.Result) {
	if perf.ClosedTrades() == 0 {
		return
	}
	wins := int32(perf.Wins)
	losses := int32(perf.Losses)
	even := int32(perf.Breakeven)
	m.Wins = &wins
	m.Losses = &losses
	m.Breakeven = &even
	m.WinRate = perf.WinRate
	if perf.MonthlyWinRate != nil {
		m.MonthlyWinRateAvg = f64Ptr(perf.MonthlyWinRate.Average)
		m.MonthlyWinRateMed = f64Ptr(perf.MonthlyWinRate.Median)
	}
	if perf.YearlyWinRate != nil {
		m.YearlyWinRateAvg = f64Ptr(perf.YearlyWinRate.Average)
		m.YearlyWinRateMed = f64Ptr(perf.YearlyWinRate.Median)
	}
}
