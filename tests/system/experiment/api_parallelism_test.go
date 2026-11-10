package experiment_test

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"

	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1/tradingcopilotv1connect"
)

// createViaAPI creates a manual-list experiment with the given settings fragment (protojson fields).
func createViaAPI(t *testing.T, client tradingcopilotv1connect.ExperimentServiceClient, settings string) (*v1.Experiment, error) {
	t.Helper()
	body := fmt.Sprintf(`{"name":"parallelism","mode":"EXPERIMENT_CREATE_MODE_MANUAL",
	  "settings":{"provider":"binance","base":"BTC","quote":"USDT","fromMs":"0","toMs":"%d","redownloadMissing":false%s},
	  "instances":[{"label":"sma 3/6","maCross":{"fastHr":3,"slowHr":6}}]}`, 240*hour, settings)
	req := &v1.CreateExperimentRequest{}
	if err := protojson.Unmarshal([]byte(body), req); err != nil {
		t.Fatal(err)
	}
	resp, err := client.CreateExperiment(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return resp.Msg.Experiment, nil
}

func parallelismClient(t *testing.T) tradingcopilotv1connect.ExperimentServiceClient {
	t.Helper()
	url := pgScratchDB(t)
	pool, err := pgxpool.New(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return newExperimentClient(serveExperimentAPI(t, pool))
}

func TestCreateExperimentParallelism(t *testing.T) {
	client := parallelismClient(t)

	// Set: persisted, returned by create and by get.
	created, err := createViaAPI(t, client, `,"parallelism":3`)
	if err != nil {
		t.Fatalf("create with parallelism 3: %v", err)
	}
	if created.GetParallelism() != 3 {
		t.Fatalf("create response parallelism: %v", created.Parallelism)
	}
	got, err := client.GetExperiment(t.Context(), connect.NewRequest(&v1.GetExperimentRequest{Id: created.Id}))
	if err != nil || got.Msg.Experiment.GetParallelism() != 3 {
		t.Fatalf("get parallelism: %v %v", err, got)
	}

	// Unset: the field is absent, as before this field existed.
	unset, err := createViaAPI(t, client, "")
	if err != nil {
		t.Fatalf("create without parallelism: %v", err)
	}
	if unset.Parallelism != nil {
		t.Fatalf("unset parallelism must stay unset: %v", *unset.Parallelism)
	}

	// Invalid values are rejected with a message.
	if _, err := createViaAPI(t, client, `,"parallelism":-1`); connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "must be 0 (auto)") {
		t.Fatalf("negative: %v", err)
	}
	above := runtime.NumCPU() + 1
	if _, err := createViaAPI(t, client, fmt.Sprintf(`,"parallelism":%d`, above)); connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "above the maximum") {
		t.Fatalf("above the maximum: %v", err)
	}

	// The limit the create form shows.
	lim, err := client.GetExperimentLimits(t.Context(), connect.NewRequest(&v1.GetExperimentLimitsRequest{}))
	if err != nil || int(lim.Msg.MaxParallelism) != runtime.NumCPU() {
		t.Fatalf("limits: %v %v", err, lim)
	}
}
