package experimentapi

import (
	"context"
	"net/http"

	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Hooks wire experiment run/cancel to the orchestrator (Docker / fan-out).
type Hooks struct {
	RunExperiment func(ctx context.Context, experimentID string, retryFailed bool)
	CancelStudy   func(experimentID string)
	// JobStatus reports the orchestrator's job for an experiment. Nil or an empty JobState when no job runs.
	JobStatus func(experimentID string) JobState
}

// Server serves the Experiment API over Postgres (sqlc).
type Server struct {
	pool  *pgxpool.Pool
	q     *experimentdb.Queries
	hooks Hooks
}

// NewServer builds a Experiment API handler set backed by pool.
func NewServer(pool *pgxpool.Pool, hooks Hooks) *Server {
	return &Server{
		pool:  pool,
		q:     experimentdb.New(pool),
		hooks: hooks,
	}
}

// Mount registers /api/v1/experiments* routes on mux.
func Mount(mux *http.ServeMux, s *Server) {
	// Every route is gzip-capable: the chart series and activity rows are large JSON or protobuf bodies.
	handle := func(mux *http.ServeMux, pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, gzipHandler(h))
	}
	handle(mux, "GET /api/v1/experiments", s.handleListExperiments)
	handle(mux, "GET /api/v1/experiments/strategies", s.handleListStrategies)
	handle(mux, "GET /api/v1/experiments/limits", s.handleExperimentLimits)
	handle(mux, "GET /api/v1/experiments/{id}/status", s.handleExperimentStatus)
	handle(mux, "POST /api/v1/experiments", s.handleCreateStudy)
	handle(mux, "GET /api/v1/experiments/{id}", s.handleGetExperiment)
	handle(mux, "DELETE /api/v1/experiments/{id}", s.handleDeleteExperiment)
	handle(mux, "GET /api/v1/experiments/{id}/progress", s.handleStudyProgress)
	handle(mux, "GET /api/v1/experiments/{id}/performance", s.handleExperimentPerformance)
	handle(mux, "GET /api/v1/experiments/{id}/instances", s.handleListInstances)
	handle(mux, "GET /api/v1/experiments/{id}/instances/{instanceId}", s.handleGetInstance)
	handle(mux, "GET /api/v1/experiments/{id}/instances/{instanceId}/chart-series", s.handleChartSeries)
	handle(mux, "GET /api/v1/experiments/{id}/instances/{instanceId}/report", s.handleInstanceReport)
	handle(mux, "GET /api/v1/experiments/{id}/instances/{instanceId}/kpis", s.handleInstanceKPIs)
	handle(mux, "GET /api/v1/experiments/{id}/instances/{instanceId}/activity", s.handleInstanceActivity)
	handle(mux, "POST /api/v1/experiments/{id}/run", s.handleRunExperiment)
	handle(mux, "POST /api/v1/experiments/{id}/cancel", s.handleCancelStudy)
}
