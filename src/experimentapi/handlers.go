package experimentapi

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Server) handleListExperiments(w http.ResponseWriter, r *http.Request) {
	rows, err := s.q.ListExperiments(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]*v1.Experiment, 0, len(rows))
	for _, row := range rows {
		pb, _ := s.q.ListDistinctPlaybooks(r.Context(), row.ID)
		sym, _ := s.q.ListDistinctSymbols(r.Context(), row.ID)
		out = append(out, experimentFromRow(row, pb, sym))
	}
	writeProto(w, http.StatusOK, &v1.ListExperimentsResponse{Experiments: out})
}

func (s *Server) handleCreateStudy(w http.ResponseWriter, r *http.Request) {
	var req v1.CreateExperimentRequest
	if err := readProto(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = "Experiment"
	}
	rec, err := s.createStudy(r.Context(), &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeProto(w, http.StatusCreated, rec)
}

func (s *Server) handleGetExperiment(w http.ResponseWriter, r *http.Request) {
	id, err := parseExperimentID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	row, err := s.q.GetExperiment(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeProto(w, http.StatusOK, experimentFromRow(row, nil, nil))
}

func (s *Server) handleDeleteExperiment(w http.ResponseWriter, r *http.Request) {
	id, err := parseExperimentID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	row, err := s.q.GetExperiment(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if row.Status == "running" || row.Status == "materializing" {
		msg := "cannot delete running experiment; cancel first"
		if row.Status == "materializing" {
			msg = "cannot delete while materializing; wait for draft"
		}
		writeError(w, http.StatusConflict, msg)
		return
	}
	if err := s.q.DeleteExperiment(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	removeExperimentFiles(id)
	writeProto(w, http.StatusOK, &v1.OkResponse{Ok: true})
}

// removeExperimentFiles deletes the experiment's directory under DATA_DIR (its instance workspaces and result shards).
// Best effort: the experiment is already gone from Postgres, so a failure is logged, not returned.
func removeExperimentFiles(id uuid.UUID) {
	root := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if root == "" {
		root = "/data"
	}
	dir := filepath.Join(root, "experiments", id.String())
	if err := os.RemoveAll(dir); err != nil {
		log.Printf("experiment %s: remove %s: %v", id, dir, err)
	}
}

func (s *Server) handleStudyProgress(w http.ResponseWriter, r *http.Request) {
	id, err := parseExperimentID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	row, err := s.q.GetExperiment(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	failed, err := s.q.CountFailedInstances(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeProto(w, http.StatusOK, progressFromRow(row, failed))
}

func (s *Server) handleListInstances(w http.ResponseWriter, r *http.Request) {
	id, err := parseExperimentID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := s.q.GetExperiment(r.Context(), id); errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	q := r.URL.Query()
	limit := int32(50)
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = int32(n)
		}
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 200 {
		limit = 200
	}
	offset := int32(0)
	if c := strings.TrimSpace(q.Get("cursor")); c != "" {
		if n, err := strconv.ParseInt(c, 10, 32); err == nil && n >= 0 {
			offset = int32(n)
		}
	}
	status := strings.TrimSpace(q.Get("status"))
	symbol := strings.TrimSpace(q.Get("symbol"))
	search := strings.TrimSpace(q.Get("q"))
	sortKey := mapInstanceSort(q.Get("sort"))
	order := strings.ToLower(strings.TrimSpace(q.Get("order")))
	if order != "desc" {
		order = "asc"
	}

	total, err := s.q.CountInstancesFiltered(r.Context(), experimentdb.CountInstancesFilteredParams{
		ExperimentID: id,
		Column2:      status,
		Column3:      symbol,
		Column4:      search,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	rows, err := s.q.ListInstancesPage(r.Context(), experimentdb.ListInstancesPageParams{
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
		writeError(w, http.StatusInternalServerError, err.Error())
		return
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
	writeProto(w, http.StatusOK, &v1.ListInstancesResponse{
		Instances:  instances,
		Total:      total,
		NextCursor: nextCursor,
		Limit:      limit,
	})
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

func (s *Server) handleGetInstance(w http.ResponseWriter, r *http.Request) {
	experimentID, err := parseExperimentID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	instID, err := uuid.Parse(r.PathValue("instanceId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid instance id")
		return
	}
	row, err := s.q.GetInstance(r.Context(), experimentdb.GetInstanceParams{ExperimentID: experimentID, ID: instID})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeProto(w, http.StatusOK, instanceFromRow(row))
}

func (s *Server) handleRunExperiment(w http.ResponseWriter, r *http.Request) {
	id, err := parseExperimentID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	row, err := s.q.GetExperiment(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if row.Status == "running" {
		writeError(w, http.StatusConflict, "already running")
		return
	}

	var body v1.RunExperimentRequest
	_ = readProto(r, &body)

	now := timeNowMs()
	if body.RetryFailed {
		failed, err := s.q.CountFailedInstances(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if failed == 0 {
			writeError(w, http.StatusBadRequest, "no failed instances to retry")
			return
		}
		if err := s.markStudyRetryFailed(r.Context(), id, now, int32(failed)); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		if err := s.q.MarkExperimentRunning(r.Context(), experimentdb.MarkExperimentRunningParams{
			ID:          id,
			StartedAtMs: pgtype.Int8{Int64: now, Valid: true},
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := s.q.MarkInstancesRunning(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	if s.hooks.RunExperiment != nil {
		// The run outlives this request: its context must not be the request's, which ends with the response.
		s.hooks.RunExperiment(context.Background(), id.String(), body.RetryFailed)
	}

	writeProto(w, http.StatusAccepted, &v1.RunExperimentResponse{Id: id.String(), Running: true})
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

func (s *Server) handleCancelStudy(w http.ResponseWriter, r *http.Request) {
	id, err := parseExperimentID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	row, err := s.q.GetExperiment(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if row.Status != "running" {
		writeError(w, http.StatusConflict, "experiment is not running")
		return
	}

	now := timeNowMs()
	if err := s.q.MarkExperimentCancelled(r.Context(), experimentdb.MarkExperimentCancelledParams{
		ID:           id,
		FinishedAtMs: pgtype.Int8{Int64: now, Valid: true},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.q.MarkRunningInstancesCancelled(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if s.hooks.CancelStudy != nil {
		s.hooks.CancelStudy(id.String())
	}

	got, err := s.q.GetExperiment(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeProto(w, http.StatusOK, experimentFromRow(got, nil, nil))
}

func parseExperimentID(raw string) (uuid.UUID, error) {
	return uuid.Parse(raw)
}
