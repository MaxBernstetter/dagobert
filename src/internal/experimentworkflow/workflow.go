// Package experimentworkflow connects the Experiment API to the Step Functions workflow that runs an
// experiment: Prepare -> Map(Simulate batches) -> Finalize. It holds the payloads both sides exchange
// and the Experiment API hooks that start, stop and describe an execution.
package experimentworkflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/aws-sdk-go-v2/service/sfn/types"

	"github.com/MaxBernstetter/dagobert/src/experimentapi"
)

// Phases of one experiment-worker invocation. PhaseFail records a crash of another phase (Lambda timeout,
// out of memory) that could not mark the experiment itself.
const (
	PhasePrepare  = "prepare"
	PhaseSimulate = "simulate"
	PhaseFinalize = "finalize"
	PhaseFail     = "fail"
)

// Input is the state machine input, written by the Experiment API when a run starts.
type Input struct {
	ExperimentID string `json:"experiment_id"`
	RetryFailed  bool   `json:"retry_failed"`
}

// WorkerRequest is the payload of one experiment-worker invocation.
type WorkerRequest struct {
	Phase        string   `json:"phase"`
	ExperimentID string   `json:"experiment_id"`
	InstanceIDs  []string `json:"instance_ids,omitempty"`
	RetryFailed  bool     `json:"retry_failed,omitempty"`
	Error        string   `json:"error,omitempty"`
}

// WorkerResponse is returned by the prepare phase: the simulate batches the Map state iterates over.
type WorkerResponse struct {
	Batches [][]string `json:"batches"`
}

// Validate rejects a request the phases cannot run.
func (r WorkerRequest) Validate() error {
	if r.ExperimentID == "" {
		return errors.New("experiment_id required")
	}
	switch r.Phase {
	case PhasePrepare, PhaseFinalize, PhaseFail:
		return nil
	case PhaseSimulate:
		if len(r.InstanceIDs) == 0 {
			return errors.New("instance_ids required for simulate")
		}
		return nil
	default:
		return fmt.Errorf("unknown phase %q", r.Phase)
	}
}

// ExecutionNamePrefix is shared by every execution of one experiment. Names must be unique per state
// machine, so each run appends its start time; the prefix finds all runs of an experiment.
func ExecutionNamePrefix(experimentID string) string {
	return experimentID + "-"
}

// ExecutionName is the execution name of one run (at most 80 characters: a UUID, a dash and milliseconds).
func ExecutionName(experimentID string, startedMs int64) string {
	return ExecutionNamePrefix(experimentID) + strconv.FormatInt(startedMs, 10)
}

// JobStateFromExecution maps a Step Functions execution to the job state the Experiment API reports.
// status is the execution status (RUNNING, SUCCEEDED, FAILED, TIMED_OUT, ABORTED); cause is its cause.
func JobStateFromExecution(status, cause string, stopped time.Time) experimentapi.JobState {
	var js experimentapi.JobState
	switch status {
	case string(types.ExecutionStatusRunning):
		js.Running = true
		return js
	case string(types.ExecutionStatusSucceeded):
	default:
		js.Error = cause
		if js.Error == "" {
			js.Error = "workflow " + strings.ToLower(status)
		}
	}
	if !stopped.IsZero() {
		js.Finished = stopped.UTC().Format(time.RFC3339)
	}
	return js
}

// SFN is the part of the Step Functions client the hooks use. *sfn.Client satisfies it.
type SFN interface {
	StartExecution(ctx context.Context, in *sfn.StartExecutionInput, optFns ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error)
	StopExecution(ctx context.Context, in *sfn.StopExecutionInput, optFns ...func(*sfn.Options)) (*sfn.StopExecutionOutput, error)
	ListExecutions(ctx context.Context, in *sfn.ListExecutionsInput, optFns ...func(*sfn.Options)) (*sfn.ListExecutionsOutput, error)
	DescribeExecution(ctx context.Context, in *sfn.DescribeExecutionInput, optFns ...func(*sfn.Options)) (*sfn.DescribeExecutionOutput, error)
}

const (
	hookTimeout = 30 * time.Second
	// statusMaxPages bounds the search for the latest run of an experiment. ListExecutions is newest first.
	statusMaxPages = 5
)

type hooks struct {
	api    SFN
	arn    string
	failer Failer
}

// Failer marks an experiment failed with a message. The API Lambda passes its Postgres pool.
type Failer interface {
	MarkExperimentFailed(ctx context.Context, experimentID, message string) error
}

// NewHooks returns the Experiment API hooks that drive the state machine arn.
// Run starts an execution, Cancel stops the running ones, JobStatus describes the latest one.
// When Run cannot start an execution it marks the experiment failed through failer.
func NewHooks(api SFN, stateMachineARN string, failer Failer) experimentapi.Hooks {
	h := &hooks{api: api, arn: stateMachineARN, failer: failer}
	return experimentapi.Hooks{
		RunExperiment: h.run,
		CancelStudy:   h.cancel,
		JobStatus:     h.status,
	}
}

func (h *hooks) run(ctx context.Context, experimentID string, retryFailed bool) {
	raw, err := json.Marshal(Input{ExperimentID: experimentID, RetryFailed: retryFailed})
	if err != nil {
		log.Printf("experiment %s: workflow input: %v", experimentID, err)
		return
	}
	ctx, cancel := context.WithTimeout(ctx, hookTimeout)
	defer cancel()
	_, err = h.api.StartExecution(ctx, &sfn.StartExecutionInput{
		StateMachineArn: aws.String(h.arn),
		Name:            aws.String(ExecutionName(experimentID, time.Now().UnixMilli())),
		Input:           aws.String(string(raw)),
	})
	if err != nil {
		// The hook has no error return, so the failure is recorded on the experiment row.
		msg := fmt.Sprintf("start workflow: %v", err)
		log.Printf("experiment %s: %s", experimentID, msg)
		failCtx, failCancel := context.WithTimeout(context.Background(), hookTimeout)
		defer failCancel()
		if ferr := h.failer.MarkExperimentFailed(failCtx, experimentID, msg); ferr != nil {
			log.Printf("experiment %s: mark failed: %v", experimentID, ferr)
		}
	}
}

func (h *hooks) cancel(experimentID string) {
	ctx, cancel := context.WithTimeout(context.Background(), hookTimeout)
	defer cancel()
	running, err := h.matching(ctx, experimentID, types.ExecutionStatusRunning, statusMaxPages)
	if err != nil {
		log.Printf("experiment %s: list running workflows: %v", experimentID, err)
		return
	}
	for _, ex := range running {
		_, err := h.api.StopExecution(ctx, &sfn.StopExecutionInput{
			ExecutionArn: ex.ExecutionArn,
			Error:        aws.String("Cancelled"),
			Cause:        aws.String("experiment cancelled"),
		})
		if err != nil {
			log.Printf("experiment %s: stop workflow %s: %v", experimentID, aws.ToString(ex.ExecutionArn), err)
		}
	}
}

func (h *hooks) status(experimentID string) experimentapi.JobState {
	ctx, cancel := context.WithTimeout(context.Background(), hookTimeout)
	defer cancel()
	running, err := h.matching(ctx, experimentID, types.ExecutionStatusRunning, statusMaxPages)
	if err != nil {
		log.Printf("experiment %s: list running workflows: %v", experimentID, err)
		return experimentapi.JobState{}
	}
	if len(running) > 0 {
		return JobStateFromExecution(string(types.ExecutionStatusRunning), "", time.Time{})
	}
	latest, err := h.matching(ctx, experimentID, "", statusMaxPages)
	if err != nil || len(latest) == 0 {
		return experimentapi.JobState{}
	}
	desc, err := h.api.DescribeExecution(ctx, &sfn.DescribeExecutionInput{ExecutionArn: latest[0].ExecutionArn})
	if err != nil {
		log.Printf("experiment %s: describe workflow: %v", experimentID, err)
		return experimentapi.JobState{}
	}
	return JobStateFromExecution(string(desc.Status), aws.ToString(desc.Cause), aws.ToTime(desc.StopDate))
}

// matching lists executions of the state machine whose name carries the experiment prefix. An empty
// filter lists every status. The result is newest first.
func (h *hooks) matching(ctx context.Context, experimentID string, filter types.ExecutionStatus, maxPages int) ([]types.ExecutionListItem, error) {
	prefix := ExecutionNamePrefix(experimentID)
	in := &sfn.ListExecutionsInput{StateMachineArn: aws.String(h.arn)}
	if filter != "" {
		in.StatusFilter = filter
	}
	var out []types.ExecutionListItem
	for page := 0; page < maxPages; page++ {
		res, err := h.api.ListExecutions(ctx, in)
		if err != nil {
			return nil, err
		}
		for _, ex := range res.Executions {
			if strings.HasPrefix(aws.ToString(ex.Name), prefix) {
				out = append(out, ex)
			}
		}
		if res.NextToken == nil || (filter == "" && len(out) > 0) {
			break
		}
		in.NextToken = res.NextToken
	}
	return out, nil
}
