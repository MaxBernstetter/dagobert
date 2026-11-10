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

	"github.com/MaxBernstetter/dagobert/src/experimentapi"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	initOnce sync.Once
	mux      *http.ServeMux
	initErr  error
)

// setup builds the database pool and the HTTP mux once per cold start.
func setup() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		initErr = errors.New("DATABASE_URL required")
		return
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		initErr = err
		return
	}
	mux = http.NewServeMux()
	experimentapi.Mount(mux, experimentapi.NewServer(pool, experimentapi.Hooks{}))
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
	mux.ServeHTTP(rec, req)
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
