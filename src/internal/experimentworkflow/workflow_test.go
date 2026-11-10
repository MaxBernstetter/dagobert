package experimentworkflow

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sfn"
)

// fakeSFN fails StartExecution; the other calls are not used by the run hook.
type fakeSFN struct {
	SFN
	err error
}

func (f fakeSFN) StartExecution(context.Context, *sfn.StartExecutionInput, ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error) {
	return nil, f.err
}

type recordFailer struct {
	id, msg string
}

func (r *recordFailer) MarkExperimentFailed(_ context.Context, id, msg string) error {
	r.id, r.msg = id, msg
	return nil
}

func TestRunMarksExperimentFailedWhenStartFails(t *testing.T) {
	failer := &recordFailer{}
	hooks := NewHooks(fakeSFN{err: errors.New("AccessDenied")}, "arn:aws:states:::stateMachine:x", failer)
	hooks.RunExperiment(context.Background(), "0b8f2c1e-7a3d-4f5e-9c6b-1d2e3f4a5b6c", false)
	if failer.id != "0b8f2c1e-7a3d-4f5e-9c6b-1d2e3f4a5b6c" {
		t.Fatalf("marked experiment %q", failer.id)
	}
	if !strings.HasPrefix(failer.msg, "start workflow: ") || !strings.Contains(failer.msg, "AccessDenied") {
		t.Fatalf("message %q", failer.msg)
	}
}

func TestJobStateFromExecution(t *testing.T) {
	stopped := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		status  string
		cause   string
		running bool
		err     string
		finish  string
	}{
		{"running", "RUNNING", "", true, "", ""},
		{"succeeded", "SUCCEEDED", "", false, "", "2026-11-10T12:00:00Z"},
		{"failed with cause", "FAILED", "prepare: coverage", false, "prepare: coverage", "2026-11-10T12:00:00Z"},
		{"failed without cause", "FAILED", "", false, "workflow failed", "2026-11-10T12:00:00Z"},
		{"cancelled", "ABORTED", "experiment cancelled", false, "experiment cancelled", "2026-11-10T12:00:00Z"},
		{"timed out", "TIMED_OUT", "", false, "workflow timed_out", "2026-11-10T12:00:00Z"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := JobStateFromExecution(tc.status, tc.cause, stopped)
			if got.Running != tc.running || got.Error != tc.err || got.Finished != tc.finish {
				t.Fatalf("got %+v, want running=%v error=%q finished=%q", got, tc.running, tc.err, tc.finish)
			}
		})
	}
}

func TestJobStateRunningHasNoFinish(t *testing.T) {
	got := JobStateFromExecution("RUNNING", "", time.Time{})
	if !got.Running || got.Finished != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestExecutionNameCarriesPrefix(t *testing.T) {
	id := "0b8f2c1e-7a3d-4f5e-9c6b-1d2e3f4a5b6c"
	name := ExecutionName(id, 1789000000123)
	if !strings.HasPrefix(name, ExecutionNamePrefix(id)) {
		t.Fatalf("name %q lacks prefix %q", name, ExecutionNamePrefix(id))
	}
	if len(name) > 80 {
		t.Fatalf("name length %d exceeds the Step Functions limit", len(name))
	}
	other := ExecutionName("1b8f2c1e-7a3d-4f5e-9c6b-1d2e3f4a5b6c", 1)
	if strings.HasPrefix(other, ExecutionNamePrefix(id)) {
		t.Fatalf("different experiment matched prefix")
	}
}

func TestWorkerRequestValidate(t *testing.T) {
	cases := []struct {
		name string
		req  WorkerRequest
		ok   bool
	}{
		{"prepare", WorkerRequest{Phase: PhasePrepare, ExperimentID: "e"}, true},
		{"finalize", WorkerRequest{Phase: PhaseFinalize, ExperimentID: "e"}, true},
		{"fail", WorkerRequest{Phase: PhaseFail, ExperimentID: "e", Error: "timeout"}, true},
		{"simulate", WorkerRequest{Phase: PhaseSimulate, ExperimentID: "e", InstanceIDs: []string{"a"}}, true},
		{"simulate without ids", WorkerRequest{Phase: PhaseSimulate, ExperimentID: "e"}, false},
		{"no experiment", WorkerRequest{Phase: PhasePrepare}, false},
		{"unknown phase", WorkerRequest{Phase: "all", ExperimentID: "e"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.req.Validate(); (err == nil) != tc.ok {
				t.Fatalf("Validate() = %v, want ok=%v", err, tc.ok)
			}
		})
	}
}
