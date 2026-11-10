package framework

import (
	"encoding/json"

	"github.com/MaxBernstetter/dagobert/src/calculation-performance"
)

// InstanceReport is the report of one experiment instance sim, stored as JSON in reports.parquet.
type InstanceReport struct {
	Version    int                `json:"version"`
	Run        RunReport          `json:"run"`
	KPIs       InstanceReportKPIs `json:"kpis"`
	Statistics RunStatistics      `json:"statistics"`
	// Performance is the in-memory win-rate result for the catalog write.
	// It is not part of the report JSON; Postgres is the source for these metrics.
	Performance performance.Result `json:"-"`
}

// InstanceReportKPIs mirrors the Strategy KPI API payload.
type InstanceReportKPIs struct {
	Playbook   string                 `json:"playbook"`
	Symbol     string                 `json:"symbol"`
	FromMs     int64                  `json:"fromMs"`
	ToMs       int64                  `json:"toMs"`
	Results    []StrategyKPIResult    `json:"results"`
	Comparison *StrategyKPIComparison `json:"comparison,omitempty"`
}

// MarshalInstanceReport is the JSON of an instance report, as stored in reports.parquet and served
// by the report endpoint.
func MarshalInstanceReport(report InstanceReport) ([]byte, error) {
	if report.Version == 0 {
		report.Version = 1
	}
	return json.MarshalIndent(report, "", "  ")
}

// BuildInstanceReportFromSim assembles KPIs + statistics after a slim/full sim.
func BuildInstanceReportFromSim(
	run RunReport,
	playbookKey, symbol string,
	fromMs, toMs int64,
	startCash float64,
	fills []StrategyKPIFill,
	bars []StrategyKPIBar,
) InstanceReport {
	portfolio, bench := ReplayEquityCurves(fills, bars, startCash)
	results := ComputeStrategyKPIValues(portfolio, bench, fromMs, toMs)
	stats := ComputeRunStatistics(portfolio, fills, fromMs, toMs)
	comp := BuildComparison(portfolio, bench, startCash)
	perf := performance.Evaluate(performanceFills(fills), fromMs, toMs)
	return InstanceReport{
		Version: 1,
		Run:     run,
		KPIs: InstanceReportKPIs{
			Playbook:   playbookKey,
			Symbol:     symbol,
			FromMs:     fromMs,
			ToMs:       toMs,
			Results:    results,
			Comparison: comp,
		},
		Statistics:  stats,
		Performance: perf,
	}
}

func performanceFills(fills []StrategyKPIFill) []performance.Fill {
	out := make([]performance.Fill, len(fills))
	for i, f := range fills {
		out[i] = performance.Fill{
			CandleCloseMs: f.CandleCloseMs,
			Side:          f.Side,
			BaseQty:       f.BaseQty,
			QuoteQty:      f.QuoteQty,
		}
	}
	return out
}
