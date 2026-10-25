package runner

import "github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"

// Re-exports for experiment job Postgres writers (backtest cannot import internal/).

type StrategyKPIFill = framework.StrategyKPIFill
type RunStatistics = framework.RunStatistics
type PeriodReturn = framework.PeriodReturn

func LoadStrategyKPIFills(dataDir, playbook string) ([]StrategyKPIFill, error) {
	return framework.LoadStrategyKPIFills(dataDir, playbook)
}
