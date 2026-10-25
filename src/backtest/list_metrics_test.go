package backtest

import (
	"encoding/json"
	"testing"

	"github.com/MaxBernstetter/dagobert/src/calculation-performance"
	signalrunner "github.com/MaxBernstetter/dagobert/src/calculation-signal/runner"
)

func TestListMetricsFromReport(t *testing.T) {
	const raw = `{
	  "version": 1,
	  "run": { "startedAtMs": 1, "finishedAtMs": 1251, "durationMs": 1250 },
	  "kpis": {
	    "playbook": "ma-cross",
	    "symbol": "BTCUSDT",
	    "fromMs": 0,
	    "toMs": 1,
	    "results": [
	      { "def": { "slug": "sharpe" }, "value": 1.25 },
	      { "def": { "slug": "cagr" }, "value": 0.12 },
	      { "def": { "slug": "max-drawdown" }, "value": 0.2 },
	      { "def": { "slug": "sortino" }, "value": 1.8 }
	    ]
	  },
	  "statistics": {
	    "durationMonths": 12,
	    "durationYears": 1,
	    "monthlyReturn": { "average": 0.01, "median": 0.009 },
	    "yearlyReturn": { "average": 0.12, "median": 0.11 },
	    "tradesPerMonth": { "average": 4.5, "median": 4 }
	  }
	}`
	var report signalrunner.InstanceReport
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		t.Fatal(err)
	}
	m := ListMetricsFromReport(report)
	if m.DurationMs == nil || *m.DurationMs != 1250 {
		t.Fatalf("duration: %+v", m.DurationMs)
	}
	if m.Sharpe == nil || *m.Sharpe != 1.25 {
		t.Fatalf("sharpe: %+v", m.Sharpe)
	}
	if m.Cagr == nil || *m.Cagr != 0.12 {
		t.Fatalf("cagr: %+v", m.Cagr)
	}
	if m.MonthlyReturnAvg == nil || *m.MonthlyReturnAvg != 0.01 {
		t.Fatalf("mo avg: %+v", m.MonthlyReturnAvg)
	}
	if m.TradesPerMonthMed == nil || *m.TradesPerMonthMed != 4 {
		t.Fatalf("trades med: %+v", m.TradesPerMonthMed)
	}
	if m.WinRate != nil {
		t.Fatalf("win rate comes from the in-memory result, not report.json: %+v", m.WinRate)
	}
	rate := 0.5
	report.Performance = performance.Result{
		Wins: 1, Losses: 1,
		WinRate:        &rate,
		MonthlyWinRate: &performance.AvgMed{Average: 0.4, Median: 0.25},
		YearlyWinRate:  &performance.AvgMed{Average: 0.5, Median: 0.5},
	}
	m = ListMetricsFromReport(report)
	if m.Wins == nil || *m.Wins != 1 || m.Losses == nil || *m.Losses != 1 {
		t.Fatalf("counts: %+v %+v", m.Wins, m.Losses)
	}
	if m.WinRate == nil || *m.WinRate != 0.5 {
		t.Fatalf("win rate: %+v", m.WinRate)
	}
	if m.MonthlyWinRateAvg == nil || *m.MonthlyWinRateAvg != 0.4 {
		t.Fatalf("month win avg: %+v", m.MonthlyWinRateAvg)
	}
	if m.YearlyWinRateMed == nil || *m.YearlyWinRateMed != 0.5 {
		t.Fatalf("year win med: %+v", m.YearlyWinRateMed)
	}
}
