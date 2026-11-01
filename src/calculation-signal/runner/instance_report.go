package runner

import (
	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
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
