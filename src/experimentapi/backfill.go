package experimentapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/aws-sdk-go-v2/service/sfn/types"

	"github.com/MaxBernstetter/dagobert/src/experimentapi/pgcatalog"
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1/tradingcopilotv1connect"
)

// BackfillSFN is the part of the Step Functions client the backfill service uses. *sfn.Client satisfies it.
type BackfillSFN interface {
	StartExecution(ctx context.Context, in *sfn.StartExecutionInput, optFns ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error)
	StopExecution(ctx context.Context, in *sfn.StopExecutionInput, optFns ...func(*sfn.Options)) (*sfn.StopExecutionOutput, error)
	ListExecutions(ctx context.Context, in *sfn.ListExecutionsInput, optFns ...func(*sfn.Options)) (*sfn.ListExecutionsOutput, error)
	DescribeExecution(ctx context.Context, in *sfn.DescribeExecutionInput, optFns ...func(*sfn.Options)) (*sfn.DescribeExecutionOutput, error)
}

// ProviderStore reads the saved provider state. *pgcatalog.Catalog satisfies it.
type ProviderStore interface {
	GetProvider(ctx context.Context, id string) (pgcatalog.ProviderRecord, bool, error)
}

const (
	backfillDefaultLimit = 20
	backfillMaxLimit     = 100
	// backfillMaxPages bounds the search for running executions of one market.
	backfillMaxPages = 10
	backfillTimeout  = 30 * time.Second
)

// BackfillServer implements tradingcopilotv1connect.BackfillServiceHandler. A backfill is one Step Functions
// execution of the backfill state machine per market; the server validates, starts, describes and stops them.
type BackfillServer struct {
	sfn        BackfillSFN
	arn        string
	providers  ProviderStore
	envBaseURL string
	now        func() time.Time
}

var _ tradingcopilotv1connect.BackfillServiceHandler = (*BackfillServer)(nil)

// NewBackfillServer serves the backfill state machine arn. envBaseURL is the BINANCE_BASE_URL value of the
// environment: it applies when no base URL is saved for the provider (the same precedence as the orchestrator).
func NewBackfillServer(api BackfillSFN, stateMachineARN string, providers ProviderStore, envBaseURL string) *BackfillServer {
	return &BackfillServer{sfn: api, arn: stateMachineARN, providers: providers, envBaseURL: envBaseURL, now: time.Now}
}

// MountBackfill serves BackfillService (Connect-RPC) on mux at /tradingcopilot.v1.BackfillService/.
func MountBackfill(mux *http.ServeMux, s *BackfillServer) {
	path, handler := tradingcopilotv1connect.NewBackfillServiceHandler(s)
	mux.Handle(path, handler)
}

// StartBackfill validates the request, refuses a market that already runs and starts one execution per market.
// The provider must be activated and ready. Markets start in request order; a failed start stops the request
// and names the executions already started.
func (s *BackfillServer) StartBackfill(ctx context.Context, req *connect.Request[v1.StartBackfillRequest]) (*connect.Response[v1.StartBackfillResponse], error) {
	ctx, cancel := context.WithTimeout(ctx, backfillTimeout)
	defer cancel()
	now := s.now()
	provider, err := ResolveBackfillProvider(req.Msg.Provider)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	markets, err := BackfillMarkets(req.Msg.Symbols)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	from, to, err := BackfillWindow(req.Msg.FromMs, req.Msg.ToMs, now.UnixMilli())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rec, ok, err := s.providers.GetProvider(ctx, provider)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok || !rec.Activated || rec.Status != "ready" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("provider %s is not ready: activate it on the Connectors page", provider))
	}
	for _, m := range markets {
		running, err := s.running(ctx, BackfillNamePrefix(provider, m.Symbol))
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if running {
			return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("a backfill of %s is already running", m.Symbol))
		}
	}
	out := &v1.StartBackfillResponse{}
	for _, m := range markets {
		raw, err := json.Marshal(BackfillInputFor(provider, m, from, to, rec.Config, s.envBaseURL))
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		name := BackfillExecutionName(provider, m.Symbol, from, to, now.UnixMilli())
		res, err := s.sfn.StartExecution(ctx, &sfn.StartExecutionInput{
			StateMachineArn: aws.String(s.arn),
			Name:            aws.String(name),
			Input:           aws.String(string(raw)),
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal,
				fmt.Errorf("start %s: %w (started: %s)", m.Symbol, err, startedList(out)))
		}
		out.Executions = append(out.Executions, &v1.BackfillExecution{
			Symbol:       m.Symbol,
			ExecutionArn: aws.ToString(res.ExecutionArn),
			Name:         name,
		})
	}
	return connect.NewResponse(out), nil
}

// GetBackfillStatus describes one execution of the backfill state machine, with its error.
func (s *BackfillServer) GetBackfillStatus(ctx context.Context, req *connect.Request[v1.GetBackfillStatusRequest]) (*connect.Response[v1.GetBackfillStatusResponse], error) {
	ctx, cancel := context.WithTimeout(ctx, backfillTimeout)
	defer cancel()
	desc, err := s.describe(ctx, req.Msg.ExecutionArn)
	if err != nil {
		return nil, err
	}
	status, errMsg := BackfillRunState(string(desc.Status), aws.ToString(desc.Cause))
	return connect.NewResponse(&v1.GetBackfillStatusResponse{Backfill: &v1.BackfillRun{
		ExecutionArn: aws.ToString(desc.ExecutionArn),
		Name:         aws.ToString(desc.Name),
		Status:       status,
		Started:      formatTime(aws.ToTime(desc.StartDate)),
		Stopped:      formatTime(aws.ToTime(desc.StopDate)),
		Error:        errMsg,
	}}), nil
}

// ListBackfills lists the newest executions of the backfill state machine, without their error.
func (s *BackfillServer) ListBackfills(ctx context.Context, req *connect.Request[v1.ListBackfillsRequest]) (*connect.Response[v1.ListBackfillsResponse], error) {
	ctx, cancel := context.WithTimeout(ctx, backfillTimeout)
	defer cancel()
	limit := int(req.Msg.Limit)
	switch {
	case limit <= 0:
		limit = backfillDefaultLimit
	case limit > backfillMaxLimit:
		limit = backfillMaxLimit
	}
	res, err := s.sfn.ListExecutions(ctx, &sfn.ListExecutionsInput{
		StateMachineArn: aws.String(s.arn),
		MaxResults:      int32(limit),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := &v1.ListBackfillsResponse{Backfills: make([]*v1.BackfillRun, 0, len(res.Executions))}
	for _, ex := range res.Executions {
		status, _ := BackfillRunState(string(ex.Status), "")
		out.Backfills = append(out.Backfills, &v1.BackfillRun{
			ExecutionArn: aws.ToString(ex.ExecutionArn),
			Name:         aws.ToString(ex.Name),
			Status:       status,
			Started:      formatTime(aws.ToTime(ex.StartDate)),
			Stopped:      formatTime(aws.ToTime(ex.StopDate)),
		})
	}
	return connect.NewResponse(out), nil
}

// CancelBackfill stops a running execution. Finished executions are not changed.
func (s *BackfillServer) CancelBackfill(ctx context.Context, req *connect.Request[v1.CancelBackfillRequest]) (*connect.Response[v1.CancelBackfillResponse], error) {
	ctx, cancel := context.WithTimeout(ctx, backfillTimeout)
	defer cancel()
	desc, err := s.describe(ctx, req.Msg.ExecutionArn)
	if err != nil {
		return nil, err
	}
	if desc.Status != types.ExecutionStatusRunning {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("backfill is %s, not running", strings.ToLower(string(desc.Status))))
	}
	if _, err := s.sfn.StopExecution(ctx, &sfn.StopExecutionInput{
		ExecutionArn: desc.ExecutionArn,
		Error:        aws.String("Cancelled"),
		Cause:        aws.String("backfill cancelled through the API"),
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&v1.CancelBackfillResponse{}), nil
}

// describe returns an execution of this state machine. Another state machine's execution is not found.
func (s *BackfillServer) describe(ctx context.Context, arn string) (*sfn.DescribeExecutionOutput, error) {
	if strings.TrimSpace(arn) == "" {
		return nil, invalidArgument("execution_arn required")
	}
	desc, err := s.sfn.DescribeExecution(ctx, &sfn.DescribeExecutionInput{ExecutionArn: aws.String(arn)})
	var missing *types.ExecutionDoesNotExist
	if errors.As(err, &missing) || (err == nil && aws.ToString(desc.StateMachineArn) != s.arn) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("backfill not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return desc, nil
}

// running reports whether an execution with the name prefix is running. Running executions are few: a
// bounded number of pages is searched.
func (s *BackfillServer) running(ctx context.Context, prefix string) (bool, error) {
	in := &sfn.ListExecutionsInput{StateMachineArn: aws.String(s.arn), StatusFilter: types.ExecutionStatusRunning}
	for page := 0; page < backfillMaxPages; page++ {
		res, err := s.sfn.ListExecutions(ctx, in)
		if err != nil {
			return false, err
		}
		for _, ex := range res.Executions {
			if strings.HasPrefix(aws.ToString(ex.Name), prefix) {
				return true, nil
			}
		}
		if res.NextToken == nil {
			return false, nil
		}
		in.NextToken = res.NextToken
	}
	return false, nil
}

func startedList(out *v1.StartBackfillResponse) string {
	if len(out.Executions) == 0 {
		return "none"
	}
	arns := make([]string, 0, len(out.Executions))
	for _, e := range out.Executions {
		arns = append(arns, e.ExecutionArn)
	}
	return strings.Join(arns, ", ")
}

// formatTime renders an execution time in RFC 3339 UTC; the zero time (not yet stopped) is empty.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
