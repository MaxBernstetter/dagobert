package experimentapi

import (
	"context"
	"log"
	"net/http"

	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/pgcatalog"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/providers"
	"github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1/tradingcopilotv1connect"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Hooks wire experiment run and cancel to the Step Functions workflow (src/internal/experimentworkflow). A nil
// hook does nothing.
type Hooks struct {
	RunExperiment func(ctx context.Context, experimentID string, retryFailed bool)
	CancelStudy   func(experimentID string)
	// JobStatus reports the workflow execution of an experiment. Nil or an empty JobState when none is known.
	JobStatus func(experimentID string) JobState
}

// Server implements tradingcopilotv1connect.ExperimentServiceHandler over Postgres (sqlc), and holds the
// provider settings service that ProviderService serves.
type Server struct {
	pool      *pgxpool.Pool
	q         *experimentdb.Queries
	hooks     Hooks
	providers *providers.Service
}

// NewServer builds the Experiment API service backed by pool. The provider settings use the same pool and the
// secret backend chosen by SECRETS_BACKEND (see providers.SecretsFromEnv).
func NewServer(pool *pgxpool.Pool, hooks Hooks) *Server {
	secrets, err := providers.SecretsFromEnv(context.Background())
	if err != nil {
		log.Printf("experimentapi: provider secrets unavailable: %v", err)
		secrets = providers.Unavailable(err)
	}
	return &Server{
		pool:      pool,
		q:         experimentdb.New(pool),
		hooks:     hooks,
		providers: providers.NewService(pgcatalog.NewFromPool(pool), secrets),
	}
}

// Mount serves ExperimentService and ProviderService (Connect-RPC, protobuf or JSON over HTTP POST) on mux at
// /tradingcopilot.v1.ExperimentService/ and /tradingcopilot.v1.ProviderService/. Connect compresses responses
// with gzip when the client accepts it: the chart series and the activity rows are large.
func Mount(mux *http.ServeMux, s *Server) {
	path, handler := tradingcopilotv1connect.NewExperimentServiceHandler(s)
	mux.Handle(path, handler)
	providerPath, providerHandler := tradingcopilotv1connect.NewProviderServiceHandler(providerServer{svc: s.providers})
	mux.Handle(providerPath, providerHandler)
}
