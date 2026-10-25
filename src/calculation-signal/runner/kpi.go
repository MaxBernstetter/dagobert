package runner

import (
	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
)

// ComputeKPIs exposes playbook performance KPIs for a data directory and optional range (ms).
func ComputeKPIs(dataDir, playbook, symbol string, fromMs, toMs int64, startCash float64) (framework.StrategyKPISet, error) {
	return framework.ComputeStrategyKPIs(dataDir, playbook, symbol, fromMs, toMs, startCash)
}
