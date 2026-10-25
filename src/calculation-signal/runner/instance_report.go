package runner

import (
	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
	"github.com/MaxBernstetter/dagobert/src/internal/kpi"
)

// InstanceReport is the filesystem artifact for one experiment instance (re-exported).
type InstanceReport = framework.InstanceReport

// WriteInstanceReport writes workspace/report.json.
func WriteInstanceReport(workspaceDir string, report InstanceReport) error {
	return framework.WriteInstanceReport(workspaceDir, report)
}

// ReadInstanceReport loads workspace/report.json.
func ReadInstanceReport(workspaceDir string) (InstanceReport, error) {
	return framework.ReadInstanceReport(workspaceDir)
}

// BuildAndWriteInstanceReport loads fills, builds KPIs/statistics, writes report.json.
// Returns the in-memory report so callers can project list metrics without re-reading the file.
func BuildAndWriteInstanceReport(
	workspaceDir string,
	run RunReport,
	playbookKey, symbol string,
	fromMs, toMs int64,
	startCash float64,
	win *kpi.Window,
	candleCloses []int64,
	aggMS int64,
) (InstanceReport, error) {
	fills, err := framework.LoadStrategyKPIFills(workspaceDir, playbookKey)
	if err != nil {
		return InstanceReport{}, err
	}
	bars := framework.BarsFromCloses(win, aggMS, candleCloses)
	report := framework.BuildInstanceReportFromSim(run, playbookKey, symbol, fromMs, toMs, startCash, fills, bars)
	if err := framework.WriteInstanceReport(workspaceDir, report); err != nil {
		return InstanceReport{}, err
	}
	return report, nil
}
