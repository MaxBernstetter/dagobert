package runner

import (
	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
)

// InstanceReport is the report of one experiment instance (re-exported).
type InstanceReport = framework.InstanceReport

// MarshalInstanceReport is the report JSON stored in reports.parquet.
func MarshalInstanceReport(report InstanceReport) ([]byte, error) {
	return framework.MarshalInstanceReport(report)
}
