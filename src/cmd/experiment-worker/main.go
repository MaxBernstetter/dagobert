// Command experiment-worker runs one phase of an experiment as an AWS Lambda. The Step Functions
// workflow invokes it: prepare (returns the simulate batches), simulate (one batch), finalize, fail.
// The phases are the same code as the experiment container (src/backtest).
package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/MaxBernstetter/dagobert/src/backtest"
	"github.com/MaxBernstetter/dagobert/src/internal/awsenv"
	"github.com/MaxBernstetter/dagobert/src/internal/experimentworkflow"
)

func main() {
	lambda.Start(handle)
}

func handle(_ context.Context, req experimentworkflow.WorkerRequest) (experimentworkflow.WorkerResponse, error) {
	if err := req.Validate(); err != nil {
		return experimentworkflow.WorkerResponse{}, err
	}
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "/tmp"
	}
	switch req.Phase {
	case experimentworkflow.PhasePrepare:
		return prepare(dataDir, req)
	case experimentworkflow.PhaseSimulate:
		return experimentworkflow.WorkerResponse{}, backtest.RunExperimentSimulate(dataDir, req.ExperimentID, req.InstanceIDs)
	case experimentworkflow.PhaseFinalize:
		return experimentworkflow.WorkerResponse{}, backtest.RunExperimentFinalize(dataDir, req.ExperimentID)
	default: // PhaseFail
		log.Printf("experiment %s: failed in workflow: %s", req.ExperimentID, req.Error)
		return experimentworkflow.WorkerResponse{}, backtest.MarkExperimentFailed(req.ExperimentID, req.Error)
	}
}

// prepare runs the prepare phase and chunks the instance list into simulate batches.
func prepare(dataDir string, req experimentworkflow.WorkerRequest) (experimentworkflow.WorkerResponse, error) {
	retry := "0"
	if req.RetryFailed {
		retry = "1"
	}
	if err := os.Setenv("EXPERIMENT_RETRY_FAILED", retry); err != nil {
		return experimentworkflow.WorkerResponse{}, err
	}
	if err := backtest.RunExperimentPrepare(dataDir, req.ExperimentID); err != nil {
		return experimentworkflow.WorkerResponse{}, err
	}
	ids, err := backtest.ReadExperimentInstanceList(dataDir, req.ExperimentID)
	if err != nil {
		return experimentworkflow.WorkerResponse{}, err
	}
	return experimentworkflow.WorkerResponse{Batches: backtest.ChunkStrings(ids, awsenv.StudyBatchSize())}, nil
}
