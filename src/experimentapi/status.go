package experimentapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// JobState is the orchestrator's view of the job that runs an experiment. The orchestrator keeps
// it in memory, so it is empty after an orchestrator restart.
type JobState struct {
	Running  bool   `json:"running"`
	Error    string `json:"error"`
	Finished string `json:"finished"`
}

// ExperimentStatus is the whole experiment in one response: progress, job state, status counts,
// and one page of instances.
type ExperimentStatus struct {
	Experiment statusExperiment `json:"experiment"`
	Job        JobState         `json:"job"`
	Total      int64            `json:"total"`
	Counts     map[string]int64 `json:"counts"`
	Instances  []statusInstance `json:"instances"`
	NextCursor *string          `json:"nextCursor"`
}

type statusExperiment struct {
	ID                    string `json:"id"`
	Status                string `json:"status"`
	Error                 string `json:"error"`
	ProgressStage         int32  `json:"progressStage"`
	ProgressStages        int32  `json:"progressStages"`
	ProgressStageLabel    string `json:"progressStageLabel"`
	ProgressInstanceDone  int32  `json:"progressInstanceDone"`
	ProgressInstanceTotal int32  `json:"progressInstanceTotal"`
	FailedInstanceCount   int64  `json:"failedInstanceCount"`
	StartedAtMs           *int64 `json:"startedAtMs"`
	FinishedAtMs          *int64 `json:"finishedAtMs"`
}

type statusInstance struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Symbol       string `json:"symbol"`
	Playbook     string `json:"playbook"`
	Status       string `json:"status"`
	Error        string `json:"error"`
	StartedAtMs  *int64 `json:"startedAtMs"`
	FinishedAtMs *int64 `json:"finishedAtMs"`
}

const (
	statusDefaultLimit = 200
	statusMaxLimit     = 1000
)

// handleExperimentStatus serves GET /api/v1/experiments/{id}/status.
func (s *Server) handleExperimentStatus(w http.ResponseWriter, r *http.Request) {
	id, err := parseExperimentID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	limit, offset := statusPage(r)

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
	counts, err := s.instanceCounts(r, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rows, err := s.q.ListInstancesPage(r.Context(), experimentdb.ListInstancesPageParams{
		ExperimentID: id,
		Column5:      mapInstanceSort(""),
		Column6:      "asc",
		Limit:        int32(limit),
		Offset:       int32(offset),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var total int64
	for _, n := range counts {
		total += n
	}
	instances := make([]statusInstance, 0, len(rows))
	for _, row := range rows {
		instances = append(instances, statusInstanceFrom(instanceFromRow(row)))
	}
	var job JobState
	if s.hooks.JobStatus != nil {
		job = s.hooks.JobStatus(id.String())
	}

	var nextCursor *string
	if int64(offset+len(instances)) < total {
		c := strconv.Itoa(offset + len(instances))
		nextCursor = &c
	}
	writeJSON(w, http.StatusOK, ExperimentStatus{
		Experiment: statusExperimentFrom(progressFromRow(row, failed)),
		Job:        job,
		Total:      total,
		Counts:     counts,
		Instances:  instances,
		NextCursor: nextCursor,
	})
}

// instanceCounts returns how many instances the experiment has in each status.
func (s *Server) instanceCounts(r *http.Request, id uuid.UUID) (map[string]int64, error) {
	rows, err := s.pool.Query(r.Context(),
		`SELECT status, COUNT(*)::bigint FROM experiment_instance WHERE experiment_id = $1 GROUP BY status`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int64{}
	for rows.Next() {
		var status string
		var n int64
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		counts[status] = n
	}
	return counts, rows.Err()
}

func statusPage(r *http.Request) (limit, offset int) {
	q := r.URL.Query()
	limit = statusDefaultLimit
	if n, err := strconv.Atoi(strings.TrimSpace(q.Get("limit"))); err == nil {
		limit = n
	}
	if limit < 1 {
		limit = 1
	}
	if limit > statusMaxLimit {
		limit = statusMaxLimit
	}
	if n, err := strconv.Atoi(strings.TrimSpace(q.Get("cursor"))); err == nil && n >= 0 {
		offset = n
	}
	return limit, offset
}

func statusExperimentFrom(p *v1.ExperimentProgress) statusExperiment {
	return statusExperiment{
		ID:                    p.GetId(),
		Status:                p.GetStatus(),
		Error:                 p.GetError(),
		ProgressStage:         p.GetProgressStage(),
		ProgressStages:        p.GetProgressStages(),
		ProgressStageLabel:    p.GetProgressStageLabel(),
		ProgressInstanceDone:  p.GetProgressInstanceDone(),
		ProgressInstanceTotal: p.GetProgressInstanceTotal(),
		FailedInstanceCount:   p.GetFailedInstanceCount(),
		StartedAtMs:           nullableMs(p.StartedAtMs),
		FinishedAtMs:          nullableMs(p.FinishedAtMs),
	}
}

func statusInstanceFrom(i *v1.ExperimentInstance) statusInstance {
	return statusInstance{
		ID:           i.GetId(),
		Label:        i.GetLabel(),
		Symbol:       i.GetSymbol(),
		Playbook:     i.GetPlaybook(),
		Status:       i.GetStatus(),
		Error:        i.GetError(),
		StartedAtMs:  nullableMs(i.StartedAtMs),
		FinishedAtMs: nullableMs(i.FinishedAtMs),
	}
}

// nullableMs maps an unset optional timestamp to JSON null.
func nullableMs(v *int64) *int64 {
	if v == nil || *v == 0 {
		return nil
	}
	return v
}
