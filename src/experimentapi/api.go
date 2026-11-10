package experimentapi

import (
	"context"
	"net/http"

	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	"github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1/tradingcopilotv1connect"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Hooks wire experiment run/cancel to the orchestrator (Docker / fan-out).
type Hooks struct {
	RunExperiment func(ctx context.Context, experimentID string, retryFailed bool)
	CancelStudy   func(experimentID string)
	// JobStatus reports the orchestrator's job for an experiment. Nil or an empty JobState when no job runs.
	JobStatus func(experimentID string) JobState
}

// Server implements tradingcopilotv1connect.ExperimentServiceHandler over Postgres (sqlc).
type Server struct {
	pool  *pgxpool.Pool
	q     *experimentdb.Queries
	hooks Hooks
}

// NewServer builds the Experiment API service backed by pool.
func NewServer(pool *pgxpool.Pool, hooks Hooks) *Server {
	return &Server{
		pool:  pool,
		q:     experimentdb.New(pool),
		hooks: hooks,
	}
}

// Mount serves ExperimentService (Connect-RPC, protobuf or JSON over HTTP POST) on mux at
// /tradingcopilot.v1.ExperimentService/. Connect compresses responses with gzip when the client accepts it:
// the chart series and the activity rows are large.
func Mount(mux *http.ServeMux, s *Server) {
	path, handler := tradingcopilotv1connect.NewExperimentServiceHandler(s)
	mux.Handle(path, handler)
}
