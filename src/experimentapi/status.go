package experimentapi

import (
	"context"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"github.com/google/uuid"
)

// JobState is the orchestrator's view of the job that runs an experiment. The orchestrator keeps
// it in memory, so it is empty after an orchestrator restart.
type JobState struct {
	Running  bool
	Error    string
	Finished string
}

const (
	statusDefaultLimit = 200
	statusMaxLimit     = 1000
)

func (s *Server) GetExperimentStatus(ctx context.Context, req *connect.Request[v1.GetExperimentStatusRequest]) (*connect.Response[v1.GetExperimentStatusResponse], error) {
	id, row, err := s.loadExperiment(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	limit, offset := statusPage(req.Msg)

	failed, err := s.q.CountFailedInstances(ctx, id)
	if err != nil {
		return nil, internal(err)
	}
	counts, err := s.instanceCounts(ctx, id)
	if err != nil {
		return nil, internal(err)
	}
	rows, err := s.q.ListInstancesPage(ctx, experimentdb.ListInstancesPageParams{
		ExperimentID: id,
		Column5:      mapInstanceSort(""),
		Column6:      "asc",
		Limit:        int32(limit),
		Offset:       int32(offset),
	})
	if err != nil {
		return nil, internal(err)
	}

	var total int64
	for _, n := range counts {
		total += n
	}
	instances := make([]*v1.ExperimentInstance, 0, len(rows))
	for _, row := range rows {
		instances = append(instances, instanceFromRow(row))
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
	return connect.NewResponse(&v1.GetExperimentStatusResponse{
		Experiment: progressFromRow(row, failed),
		Job:        &v1.ExperimentJob{Running: job.Running, Error: job.Error, Finished: job.Finished},
		Total:      total,
		Counts:     counts,
		Instances:  instances,
		NextCursor: nextCursor,
	}), nil
}

// instanceCounts returns how many instances the experiment has in each status.
func (s *Server) instanceCounts(ctx context.Context, id uuid.UUID) (map[string]int64, error) {
	rows, err := s.pool.Query(ctx,
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

func statusPage(req *v1.GetExperimentStatusRequest) (limit, offset int) {
	limit = statusDefaultLimit
	if req.Limit != 0 {
		limit = int(req.Limit)
	}
	limit = min(max(limit, 1), statusMaxLimit)
	if n, err := strconv.Atoi(strings.TrimSpace(req.Cursor)); err == nil && n >= 0 {
		offset = n
	}
	return limit, offset
}
