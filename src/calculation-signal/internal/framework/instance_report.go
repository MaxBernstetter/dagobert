package framework

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/MaxBernstetter/dagobert/src/calculation-performance"
)

const InstanceReportFileName = "report.json"

// InstanceReport is the filesystem artifact for one experiment instance sim.
type InstanceReport struct {
	Version    int                `json:"version"`
	Run        RunReport          `json:"run"`
	KPIs       InstanceReportKPIs `json:"kpis"`
	Statistics RunStatistics      `json:"statistics"`
	// Performance is the in-memory win-rate result for the catalog write.
	// It is not part of report.json; Postgres is the source for these metrics.
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

// WriteInstanceReport writes report.json under workspaceDir.
func WriteInstanceReport(workspaceDir string, report InstanceReport) error {
	if report.Version == 0 {
		report.Version = 1
	}
	body, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(workspaceDir, InstanceReportFileName)
	return os.WriteFile(path, body, 0o644)
}

// ReadInstanceReport loads report.json when present.
func ReadInstanceReport(workspaceDir string) (InstanceReport, error) {
	path := filepath.Join(workspaceDir, InstanceReportFileName)
	body, err := os.ReadFile(path)
	if err != nil {
		return InstanceReport{}, err
	}
	var report InstanceReport
	if err := json.Unmarshal(body, &report); err != nil {
		return InstanceReport{}, err
	}
	return report, nil
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
