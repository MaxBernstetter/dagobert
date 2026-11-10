package experimentapi

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"github.com/MaxBernstetter/dagobert/src/internal/workspace"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// The ExperimentService methods map failures to Connect codes: a missing experiment or instance is
// CodeNotFound, a malformed id or argument CodeInvalidArgument, an action the experiment's state
// does not allow CodeFailedPrecondition, and everything else CodeInternal.

func notFound(msg string) error {
	return connect.NewError(connect.CodeNotFound, errors.New(msg))
}

func invalidArgument(msg string) error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New(msg))
}

func failedPrecondition(msg string) error {
	return connect.NewError(connect.CodeFailedPrecondition, errors.New(msg))
}

func internal(err error) error {
	return connect.NewError(connect.CodeInternal, err)
}

// loadExperiment parses the experiment id and loads its row.
func (s *Server) loadExperiment(ctx context.Context, rawID string) (uuid.UUID, experimentdb.Experiment, error) {
	id, err := parseExperimentID(rawID)
	if err != nil {
		return uuid.Nil, experimentdb.Experiment{}, invalidArgument("invalid id")
	}
	row, err := s.q.GetExperiment(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, experimentdb.Experiment{}, notFound("not found")
	}
	if err != nil {
		return uuid.Nil, experimentdb.Experiment{}, internal(err)
	}
	return id, row, nil
}

func (s *Server) ListExperiments(ctx context.Context, _ *connect.Request[v1.ListExperimentsRequest]) (*connect.Response[v1.ListExperimentsResponse], error) {
	rows, err := s.q.ListExperiments(ctx)
	if err != nil {
		return nil, internal(err)
	}
	out := make([]*v1.Experiment, 0, len(rows))
	for _, row := range rows {
		pb, _ := s.q.ListDistinctPlaybooks(ctx, row.ID)
		sym, _ := s.q.ListDistinctSymbols(ctx, row.ID)
		out = append(out, experimentFromRow(row, pb, sym))
	}
	return connect.NewResponse(&v1.ListExperimentsResponse{Experiments: out}), nil
}

func (s *Server) CreateExperiment(ctx context.Context, req *connect.Request[v1.CreateExperimentRequest]) (*connect.Response[v1.CreateExperimentResponse], error) {
	msg := req.Msg
	if strings.TrimSpace(msg.Name) == "" {
		msg.Name = "Experiment"
	}
	rec, err := s.createStudy(ctx, msg)
	if err != nil {
		return nil, invalidArgument(err.Error())
	}
	return connect.NewResponse(&v1.CreateExperimentResponse{Experiment: rec}), nil
}

func (s *Server) GetExperiment(ctx context.Context, req *connect.Request[v1.GetExperimentRequest]) (*connect.Response[v1.GetExperimentResponse], error) {
	_, row, err := s.loadExperiment(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.GetExperimentResponse{Experiment: experimentFromRow(row, nil, nil)}), nil
}

func (s *Server) DeleteExperiment(ctx context.Context, req *connect.Request[v1.DeleteExperimentRequest]) (*connect.Response[v1.DeleteExperimentResponse], error) {
	id, row, err := s.loadExperiment(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if row.Status == "running" || row.Status == "materializing" {
		msg := "cannot delete running experiment; cancel first"
		if row.Status == "materializing" {
			msg = "cannot delete while materializing; wait for draft"
		}
		return nil, failedPrecondition(msg)
	}
	if err := s.q.DeleteExperiment(ctx, id); err != nil {
		return nil, internal(err)
	}
	removeExperimentFiles(ctx, id)
	return connect.NewResponse(&v1.DeleteExperimentResponse{}), nil
}

// removeExperimentFiles deletes everything of the experiment under DATA_DIR or, with LAKE_BACKEND=s3,
// under the bucket prefix experiments/<id>/ (its prepare cache and result shards).
// Best effort: the experiment is already gone from Postgres, so a failure is logged, not returned.
func removeExperimentFiles(ctx context.Context, id uuid.UUID) {
	if err := workspace.DeletePrefix(ctx, dataDir(), "experiments/"+id.String()); err != nil {
		log.Printf("experiment %s: remove files: %v", id, err)
	}
}

func (s *Server) GetExperimentProgress(ctx context.Context, req *connect.Request[v1.GetExperimentProgressRequest]) (*connect.Response[v1.GetExperimentProgressResponse], error) {
	id, row, err := s.loadExperiment(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	failed, err := s.q.CountFailedInstances(ctx, id)
	if err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(&v1.GetExperimentProgressResponse{Progress: progressFromRow(row, failed)}), nil
}

func (s *Server) ListInstances(ctx context.Context, req *connect.Request[v1.ListInstancesRequest]) (*connect.Response[v1.ListInstancesResponse], error) {
	msg := req.Msg
	id, _, err := s.loadExperiment(ctx, msg.ExperimentId)
	if err != nil {
		return nil, err
	}

	limit := msg.Limit
	if limit == 0 {
		limit = 50
	}
	limit = min(max(limit, 1), 200)
	offset := int32(0)
	if n, err := strconv.ParseInt(strings.TrimSpace(msg.Cursor), 10, 32); err == nil && n >= 0 {
		offset = int32(n)
	}
	status := strings.TrimSpace(msg.Status)
	symbol := strings.TrimSpace(msg.Symbol)
	search := strings.TrimSpace(msg.Q)
	sortKey := mapInstanceSort(msg.Sort)
	order := strings.ToLower(strings.TrimSpace(msg.Order))
	if order != "desc" {
		order = "asc"
	}

	total, err := s.q.CountInstancesFiltered(ctx, experimentdb.CountInstancesFilteredParams{
		ExperimentID: id,
		Column2:      status,
		Column3:      symbol,
		Column4:      search,
	})
	if err != nil {
		return nil, internal(err)
	}

	rows, err := s.q.ListInstancesPage(ctx, experimentdb.ListInstancesPageParams{
		ExperimentID: id,
		Column2:      status,
		Column3:      symbol,
		Column4:      search,
		Column5:      sortKey,
		Column6:      order,
		Limit:        limit,
		Offset:       offset,
	})
	if err != nil {
		return nil, internal(err)
	}
	instances := make([]*v1.ExperimentInstance, 0, len(rows))
	for _, row := range rows {
		instances = append(instances, instanceFromRow(row))
	}
	nextOffset := offset + int32(len(instances))
	var nextCursor *string
	if int64(nextOffset) < total {
		c := strconv.Itoa(int(nextOffset))
		nextCursor = &c
	}
	return connect.NewResponse(&v1.ListInstancesResponse{
		Instances:  instances,
		Total:      total,
		NextCursor: nextCursor,
		Limit:      limit,
	}), nil
}

func mapInstanceSort(sort string) string {
	switch strings.TrimSpace(sort) {
	case "label", "config", "sharpe", "sortino", "cagr", "maxDrawdown", "max-drawdown", "durationMs", "duration", "status", "symbol",
		"monthlyReturnAvg", "monthlyReturnMed", "yearlyReturnAvg", "yearlyReturnMed", "tradesPerMonthAvg", "tradesPerMonthMed",
		"winRate", "monthlyWinRateAvg", "monthlyWinRateMed", "yearlyWinRateAvg", "yearlyWinRateMed":
		if sort == "max-drawdown" {
			return "maxDrawdown"
		}
		if sort == "duration" {
			return "durationMs"
		}
		return strings.TrimSpace(sort)
	default:
		return ""
	}
}

func (s *Server) GetInstance(ctx context.Context, req *connect.Request[v1.GetInstanceRequest]) (*connect.Response[v1.GetInstanceResponse], error) {
	inst, err := s.loadInstance(ctx, req.Msg.ExperimentId, req.Msg.InstanceId)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.GetInstanceResponse{Instance: instanceFromRow(inst)}), nil
}

func (s *Server) RunExperiment(ctx context.Context, req *connect.Request[v1.RunExperimentRequest]) (*connect.Response[v1.RunExperimentResponse], error) {
	id, row, err := s.loadExperiment(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if row.Status == "running" {
		return nil, failedPrecondition("already running")
	}

	now := timeNowMs()
	if req.Msg.RetryFailed {
		failed, err := s.q.CountFailedInstances(ctx, id)
		if err != nil {
			return nil, internal(err)
		}
		if failed == 0 {
			return nil, failedPrecondition("no failed instances to retry")
		}
		if err := s.markStudyRetryFailed(ctx, id, now, int32(failed)); err != nil {
			return nil, internal(err)
		}
	} else {
		if err := s.q.MarkExperimentRunning(ctx, experimentdb.MarkExperimentRunningParams{
			ID:          id,
			StartedAtMs: pgtype.Int8{Int64: now, Valid: true},
		}); err != nil {
			return nil, internal(err)
		}
		if err := s.q.MarkInstancesRunning(ctx, id); err != nil {
			return nil, internal(err)
		}
	}

	if s.hooks.RunExperiment != nil {
		// The run outlives this request: its context must not be the request's, which ends with the response.
		s.hooks.RunExperiment(context.Background(), id.String(), req.Msg.RetryFailed)
	}

	return connect.NewResponse(&v1.RunExperimentResponse{Id: id.String(), Running: true}), nil
}

func (s *Server) markStudyRetryFailed(ctx context.Context, id uuid.UUID, now int64, failedCount int32) error {
	if err := s.q.MarkExperimentRunning(ctx, experimentdb.MarkExperimentRunningParams{
		ID:          id,
		StartedAtMs: pgtype.Int8{Int64: now, Valid: true},
	}); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE experiment SET progress_instance_total = $2 WHERE id = $1`,
		id, failedCount)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE experiment_instance SET status = 'running', error = ''
		WHERE experiment_id = $1 AND status = 'failed'`, id)
	return err
}

func (s *Server) CancelExperiment(ctx context.Context, req *connect.Request[v1.CancelExperimentRequest]) (*connect.Response[v1.CancelExperimentResponse], error) {
	id, row, err := s.loadExperiment(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if row.Status != "running" {
		return nil, failedPrecondition("experiment is not running")
	}

	now := timeNowMs()
	if err := s.q.MarkExperimentCancelled(ctx, experimentdb.MarkExperimentCancelledParams{
		ID:           id,
		FinishedAtMs: pgtype.Int8{Int64: now, Valid: true},
	}); err != nil {
		return nil, internal(err)
	}
	if err := s.q.MarkRunningInstancesCancelled(ctx, id); err != nil {
		return nil, internal(err)
	}

	if s.hooks.CancelStudy != nil {
		s.hooks.CancelStudy(id.String())
	}

	got, err := s.q.GetExperiment(ctx, id)
	if err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(&v1.CancelExperimentResponse{Id: id.String(), Experiment: experimentFromRow(got, nil, nil)}), nil
}

func parseExperimentID(raw string) (uuid.UUID, error) {
	return uuid.Parse(raw)
}
