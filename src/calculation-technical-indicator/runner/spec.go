package runner

import (
	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/job"
	"github.com/MaxBernstetter/dagobert/src/shared"
)

// KPIAlgo is the indicator algorithm version stored with each KPI revision.
const KPIAlgo = job.Algo

// KPIAlgoSpec records algo parameters for a data source.
func KPIAlgoSpec(source shared.DataSource) string {
	return job.AlgoSpec(source)
}
