package runner

import (
	"github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/store"
	"github.com/MaxBernstetter/trading-copilot/src/shared"
)

// KPIAlgo is the indicator algorithm version stored with each KPI revision.
const KPIAlgo = store.Algo

// KPIAlgoSpec records algo parameters for a data source.
func KPIAlgoSpec(source shared.DataSource) string {
	return store.AlgoSpec(source)
}
