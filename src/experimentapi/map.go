package experimentapi

import (
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/persist"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
)

func experimentFromRow(s experimentdb.Experiment, listPlaybooks, listSymbols []string) *v1.Experiment {
	return persist.ExperimentFromDB(s, listPlaybooks, listSymbols)
}

func instanceFromRow(i experimentdb.ExperimentInstance) *v1.ExperimentInstance {
	return persist.InstanceFromDB(i)
}

func progressFromRow(s experimentdb.Experiment, failedCount int64) *v1.ExperimentProgress {
	return persist.ProgressFromDB(s, failedCount)
}
