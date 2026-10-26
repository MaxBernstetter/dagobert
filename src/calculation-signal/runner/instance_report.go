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

// LoadInstanceFills reads the fill journal rows of one playbook workspace.
func LoadInstanceFills(workspaceDir, playbookKey string) ([]StrategyKPIFill, error) {
	return framework.LoadStrategyKPIFills(workspaceDir, playbookKey)
}

// BuildInstanceReport builds KPIs/statistics from already-loaded fills without touching disk.
func BuildInstanceReport(
	run RunReport,
	playbookKey, symbol string,
	fromMs, toMs int64,
	startCash float64,
	fills []StrategyKPIFill,
	win *kpi.Window,
	candleCloses []int64,
	aggMS int64,
) InstanceReport {
	bars := framework.BarsFromCloses(win, aggMS, candleCloses)
	return framework.BuildInstanceReportFromSim(run, playbookKey, symbol, fromMs, toMs, startCash, fills, bars)
}

// BuildAndWriteInstanceReport loads fills, builds KPIs/statistics, writes report.json.
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
	report := BuildInstanceReport(run, playbookKey, symbol, fromMs, toMs, startCash, fills, win, candleCloses, aggMS)
	if err := framework.WriteInstanceReport(workspaceDir, report); err != nil {
		return InstanceReport{}, err
	}
	return report, nil
}
