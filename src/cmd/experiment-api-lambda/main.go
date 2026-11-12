// Command experiment-api-lambda serves the Experiment API (Connect-RPC ExperimentService) as an AWS Lambda
// behind an API Gateway HTTP API (payload format 2.0).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"time"

	"github.com/MaxBernstetter/dagobert/src/experimentapi"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/pgcatalog"
	"github.com/MaxBernstetter/dagobert/src/internal/auth"
	"github.com/MaxBernstetter/dagobert/src/internal/awsenv"
	"github.com/MaxBernstetter/dagobert/src/internal/experimentworkflow"
	"github.com/MaxBernstetter/dagobert/src/internal/pgpool"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
)

var (
	initOnce sync.Once
	mux      *http.ServeMux
	api      http.Handler // mux behind sign-in
	initErr  error
)

// setup builds the database pool, the workflow hooks (when EXPERIMENT_SFN_ARN is set) and the HTTP mux once per cold start.
func setup() {
	authn, err := auth.New(auth.Config{Endpoint: cognitoEndpoint(), ClientID: os.Getenv("COGNITO_CLIENT_ID")})
	if err != nil {
		initErr = err
		return
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		initErr = errors.New("DATABASE_URL required")
		return
	}
	pool, err := pgpool.Open(context.Background(), dbURL)
	if err != nil {
		initErr = err
		return
	}
	hooks := experimentapi.Hooks{}
	expARN, backfillARN := awsenv.ExperimentSFNARN(), awsenv.BackfillSFNARN()
	var client *sfn.Client
	if expARN != "" || backfillARN != "" {
		cfg, err := awsenv.Load(context.Background())
		if err != nil {
			initErr = err
			return
		}
		client = sfn.NewFromConfig(cfg, func(o *sfn.Options) {
			if ep := awsenv.ResolveEndpoint(); ep != nil {
				o.BaseEndpoint = ep
			}
		})
	}
	if expARN != "" {
		hooks = experimentworkflow.NewHooks(client, expARN, dbFailer{q: experimentdb.New(pool)})
	}
	mux = http.NewServeMux()
	api = authn.Wrap(mux)
	experimentapi.Mount(mux, experimentapi.NewServer(pool, hooks))
	if backfillARN != "" {
		// BINANCE_BASE_URL is the environment fallback of the provider base URL (set from the SSM value in Terraform).
		experimentapi.MountBackfill(mux, experimentapi.NewBackfillServer(client, backfillARN,
			pgcatalog.NewFromPool(pool), os.Getenv("BINANCE_BASE_URL")))
	}
}

// cognitoEndpoint is Floci's URL when AWS_ENDPOINT_URL is set, else the regional Cognito API (reached through the
// cognito-idp VPC endpoint in prod).
func cognitoEndpoint() string {
	if ep := awsenv.Endpoint(); ep != "" {
		return ep + "/"
	}
	return "https://cognito-idp." + os.Getenv("AWS_REGION") + ".amazonaws.com/"
}

// dbFailer marks an experiment failed in Postgres (the workflow hook's failure path).
type dbFailer struct {
	q *experimentdb.Queries
}

func (f dbFailer) MarkExperimentFailed(ctx context.Context, experimentID, message string) error {
	id, err := uuid.Parse(experimentID)
	if err != nil {
		return err
	}
	return f.q.MarkExperimentFailed(ctx, experimentdb.MarkExperimentFailedParams{
		ID:           id,
		Error:        message,
		FinishedAtMs: pgtype.Int8{Int64: time.Now().UnixMilli(), Valid: true},
	})
}

func handle(ctx context.Context, ev events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	initOnce.Do(setup)
	if initErr != nil {
		return jsonError(http.StatusInternalServerError, "experiment-api: "+initErr.Error()), nil
	}
	req, err := toHTTPRequest(ctx, ev)
	if err != nil {
		return jsonError(http.StatusBadRequest, "invalid request body: "+err.Error()), nil
	}
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	return toAPIResponse(rec), nil
}

func jsonError(status int, msg string) events.APIGatewayV2HTTPResponse {
	body, _ := json.Marshal(map[string]string{"code": http.StatusText(status), "message": msg})
	return events.APIGatewayV2HTTPResponse{
		StatusCode: status,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(body),
	}
}

func main() {
	lambda.Start(handle)
}
