package experimentapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/aws-sdk-go-v2/service/sfn/types"

	"github.com/MaxBernstetter/dagobert/src/experimentapi/pgcatalog"
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1/tradingcopilotv1connect"
)

const (
	testSMARN   = "arn:aws:states:eu-central-1:000000000000:stateMachine:dagobert-backfill"
	testNowMs   = int64(1_800_000_000_000) // 2027-01-15T08:00:00Z, hour aligned
	testFromMs  = testNowMs - 30*24*testHour
	testBaseURL = "https://mirror.example"
)

// fakeSFN keeps executions in memory. Running executions are listed for the RUNNING filter.
type fakeSFN struct {
	mu       sync.Mutex
	started  []*sfn.StartExecutionInput
	all      []types.ExecutionListItem
	desc     map[string]*sfn.DescribeExecutionOutput
	stopped  []string
	startErr error
}

func (f *fakeSFN) StartExecution(_ context.Context, in *sfn.StartExecutionInput, _ ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.startErr != nil {
		return nil, f.startErr
	}
	f.started = append(f.started, in)
	arn := testSMARN[:len(testSMARN)-len("dagobert-backfill")] + "execution:" + aws.ToString(in.Name)
	f.all = append(f.all, types.ExecutionListItem{
		ExecutionArn: aws.String(arn), Name: in.Name, Status: types.ExecutionStatusRunning,
		StateMachineArn: aws.String(testSMARN), StartDate: aws.Time(time.UnixMilli(testNowMs)),
	})
	return &sfn.StartExecutionOutput{ExecutionArn: aws.String(arn)}, nil
}

func (f *fakeSFN) StopExecution(_ context.Context, in *sfn.StopExecutionInput, _ ...func(*sfn.Options)) (*sfn.StopExecutionOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = append(f.stopped, aws.ToString(in.ExecutionArn))
	return &sfn.StopExecutionOutput{}, nil
}

func (f *fakeSFN) ListExecutions(_ context.Context, in *sfn.ListExecutionsInput, _ ...func(*sfn.Options)) (*sfn.ListExecutionsOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []types.ExecutionListItem
	for _, ex := range f.all {
		if in.StatusFilter == "" || ex.Status == in.StatusFilter {
			out = append(out, ex)
		}
	}
	return &sfn.ListExecutionsOutput{Executions: out}, nil
}

func (f *fakeSFN) DescribeExecution(_ context.Context, in *sfn.DescribeExecutionInput, _ ...func(*sfn.Options)) (*sfn.DescribeExecutionOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d, ok := f.desc[aws.ToString(in.ExecutionArn)]
	if !ok {
		return nil, &types.ExecutionDoesNotExist{}
	}
	return d, nil
}

type fakeProviders struct {
	rec pgcatalog.ProviderRecord
	ok  bool
}

func (p fakeProviders) GetProvider(context.Context, string) (pgcatalog.ProviderRecord, bool, error) {
	return p.rec, p.ok, nil
}

func readyBinance() fakeProviders {
	return fakeProviders{ok: true, rec: pgcatalog.ProviderRecord{
		ID: "binance", Activated: true, Status: "ready", Config: map[string]string{"baseUrl": testBaseURL},
	}}
}

// backfillClient serves the BackfillService on httptest and returns the generated client.
func backfillClient(t *testing.T, api *fakeSFN, providers ProviderStore) tradingcopilotv1connect.BackfillServiceClient {
	t.Helper()
	s := NewBackfillServer(api, testSMARN, providers, "https://env.example")
	s.now = func() time.Time { return time.UnixMilli(testNowMs) }
	mux := http.NewServeMux()
	MountBackfill(mux, s)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return tradingcopilotv1connect.NewBackfillServiceClient(srv.Client(), srv.URL)
}

func connectCode(err error) connect.Code {
	return connect.CodeOf(err)
}

func TestStartBackfillStartsOneExecutionPerMarket(t *testing.T) {
	api := &fakeSFN{}
	client := backfillClient(t, api, readyBinance())

	resp, err := client.StartBackfill(t.Context(), connect.NewRequest(&v1.StartBackfillRequest{
		Provider: "binance", Symbols: []string{"BTCUSDT", "ETHUSDT"}, FromMs: testFromMs, ToMs: testNowMs + 5*testHour,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if n := len(resp.Msg.Executions); n != 2 {
		t.Fatalf("executions %d want 2", n)
	}
	if len(api.started) != 2 {
		t.Fatalf("started %d want 2", len(api.started))
	}
	var in BackfillInput
	if err := json.Unmarshal([]byte(aws.ToString(api.started[1].Input)), &in); err != nil {
		t.Fatal(err)
	}
	want := BackfillInput{Provider: "binance", Base: "ETH", Quote: "USDT", From: testFromMs, To: testNowMs, BinanceBaseURL: testBaseURL}
	if in != want {
		t.Fatalf("execution input %+v want %+v", in, want)
	}
	if aws.ToString(api.started[0].StateMachineArn) != testSMARN {
		t.Fatalf("state machine %s", aws.ToString(api.started[0].StateMachineArn))
	}
}

func TestStartBackfillRejections(t *testing.T) {
	notReady := readyBinance()
	notReady.rec.Activated = false
	notReady.rec.Status = "setting_up"
	cases := []struct {
		name      string
		providers ProviderStore
		req       *v1.StartBackfillRequest
		want      connect.Code
	}{
		{"unknown provider", readyBinance(), &v1.StartBackfillRequest{Provider: "kraken", FromMs: testFromMs, ToMs: testNowMs}, connect.CodeInvalidArgument},
		{"unknown symbol", readyBinance(), &v1.StartBackfillRequest{Symbols: []string{"DOGEUSDT"}, FromMs: testFromMs, ToMs: testNowMs}, connect.CodeInvalidArgument},
		{"empty range", readyBinance(), &v1.StartBackfillRequest{FromMs: testNowMs, ToMs: testNowMs}, connect.CodeInvalidArgument},
		{"provider not ready", notReady, &v1.StartBackfillRequest{FromMs: testFromMs, ToMs: testNowMs}, connect.CodeFailedPrecondition},
		{"provider never saved", fakeProviders{}, &v1.StartBackfillRequest{FromMs: testFromMs, ToMs: testNowMs}, connect.CodeFailedPrecondition},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			api := &fakeSFN{}
			client := backfillClient(t, api, c.providers)
			_, err := client.StartBackfill(t.Context(), connect.NewRequest(c.req))
			if got := connectCode(err); got != c.want {
				t.Fatalf("code %v want %v (%v)", got, c.want, err)
			}
			if len(api.started) != 0 {
				t.Fatalf("rejected request started %d executions", len(api.started))
			}
		})
	}
}

func TestStartBackfillRefusesRunningMarket(t *testing.T) {
	api := &fakeSFN{}
	client := backfillClient(t, api, readyBinance())
	req := &v1.StartBackfillRequest{Symbols: []string{"BTCUSDT"}, FromMs: testFromMs, ToMs: testNowMs}
	if _, err := client.StartBackfill(t.Context(), connect.NewRequest(req)); err != nil {
		t.Fatal(err)
	}
	_, err := client.StartBackfill(t.Context(), connect.NewRequest(req))
	if connectCode(err) != connect.CodeAlreadyExists {
		t.Fatalf("second start of a running market: %v", err)
	}
	if len(api.started) != 1 {
		t.Fatalf("started %d want 1", len(api.started))
	}
}

func TestGetBackfillStatusReportsErrorAndOwnership(t *testing.T) {
	failedArn := testSMARN[:len(testSMARN)-len("dagobert-backfill")] + "execution:failed"
	foreignArn := "arn:aws:states:eu-central-1:000000000000:execution:other:x"
	api := &fakeSFN{desc: map[string]*sfn.DescribeExecutionOutput{
		failedArn: {
			ExecutionArn: aws.String(failedArn), Name: aws.String("failed"), Status: types.ExecutionStatusFailed,
			StateMachineArn: aws.String(testSMARN), Cause: aws.String("The indicator task failed"),
			StartDate: aws.Time(time.Date(2027, 1, 15, 8, 0, 0, 0, time.UTC)),
			StopDate:  aws.Time(time.Date(2027, 1, 15, 8, 5, 0, 0, time.UTC)),
		},
		foreignArn: {ExecutionArn: aws.String(foreignArn), Status: types.ExecutionStatusSucceeded, StateMachineArn: aws.String("arn:other")},
	}}
	client := backfillClient(t, api, readyBinance())

	resp, err := client.GetBackfillStatus(t.Context(), connect.NewRequest(&v1.GetBackfillStatusRequest{ExecutionArn: failedArn}))
	if err != nil {
		t.Fatal(err)
	}
	b := resp.Msg.Backfill
	if b.Status != "FAILED" || b.Error != "The indicator task failed" || b.Stopped != "2027-01-15T08:05:00Z" {
		t.Fatalf("status %+v", b)
	}

	_, err = client.GetBackfillStatus(t.Context(), connect.NewRequest(&v1.GetBackfillStatusRequest{ExecutionArn: foreignArn}))
	if connectCode(err) != connect.CodeNotFound {
		t.Fatalf("execution of another state machine: %v", err)
	}
}

func TestCancelBackfillStopsOnlyRunningExecutions(t *testing.T) {
	runArn := testSMARN[:len(testSMARN)-len("dagobert-backfill")] + "execution:run"
	doneArn := testSMARN[:len(testSMARN)-len("dagobert-backfill")] + "execution:done"
	api := &fakeSFN{desc: map[string]*sfn.DescribeExecutionOutput{
		runArn:  {ExecutionArn: aws.String(runArn), Status: types.ExecutionStatusRunning, StateMachineArn: aws.String(testSMARN)},
		doneArn: {ExecutionArn: aws.String(doneArn), Status: types.ExecutionStatusSucceeded, StateMachineArn: aws.String(testSMARN)},
	}}
	client := backfillClient(t, api, readyBinance())

	if _, err := client.CancelBackfill(t.Context(), connect.NewRequest(&v1.CancelBackfillRequest{ExecutionArn: doneArn})); connectCode(err) != connect.CodeFailedPrecondition {
		t.Fatalf("cancel of a finished backfill: %v", err)
	}
	if _, err := client.CancelBackfill(t.Context(), connect.NewRequest(&v1.CancelBackfillRequest{ExecutionArn: runArn})); err != nil {
		t.Fatal(err)
	}
	if len(api.stopped) != 1 || api.stopped[0] != runArn {
		t.Fatalf("stopped %v", api.stopped)
	}
}

func TestListBackfillsLimit(t *testing.T) {
	api := &fakeSFN{}
	client := backfillClient(t, api, readyBinance())
	if _, err := client.StartBackfill(t.Context(), connect.NewRequest(&v1.StartBackfillRequest{FromMs: testFromMs, ToMs: testNowMs})); err != nil {
		t.Fatal(err)
	}
	resp, err := client.ListBackfills(t.Context(), connect.NewRequest(&v1.ListBackfillsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Msg.Backfills) != 2 || resp.Msg.Backfills[0].Status != "RUNNING" {
		t.Fatalf("list %+v", resp.Msg.Backfills)
	}
}
