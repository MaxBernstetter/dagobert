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
	mux.HandleFunc("GET /api/v1/experiments", s.handleListExperiments)
	mux.HandleFunc("GET /api/v1/experiments/strategies", s.handleListStrategies)
	mux.HandleFunc("GET /api/v1/experiments/{id}/status", s.handleExperimentStatus)
	mux.HandleFunc("POST /api/v1/experiments", s.handleCreateStudy)
	mux.HandleFunc("GET /api/v1/experiments/{id}", s.handleGetExperiment)
	mux.HandleFunc("DELETE /api/v1/experiments/{id}", s.handleDeleteExperiment)
	mux.HandleFunc("GET /api/v1/experiments/{id}/progress", s.handleStudyProgress)
	mux.HandleFunc("GET /api/v1/experiments/{id}/performance", s.handleExperimentPerformance)
	mux.HandleFunc("GET /api/v1/experiments/{id}/instances", s.handleListInstances)
	mux.HandleFunc("GET /api/v1/experiments/{id}/instances/{instanceId}", s.handleGetInstance)
	mux.HandleFunc("GET /api/v1/experiments/{id}/instances/{instanceId}/chart-series", s.handleChartSeries)
	mux.HandleFunc("GET /api/v1/experiments/{id}/instances/{instanceId}/report", s.handleInstanceReport)
	mux.HandleFunc("GET /api/v1/experiments/{id}/instances/{instanceId}/kpis", s.handleInstanceKPIs)
	mux.HandleFunc("GET /api/v1/experiments/{id}/instances/{instanceId}/activity", s.handleInstanceActivity)
	mux.HandleFunc("POST /api/v1/experiments/{id}/run", s.handleRunExperiment)
	mux.HandleFunc("POST /api/v1/experiments/{id}/cancel", s.handleCancelStudy)
}
