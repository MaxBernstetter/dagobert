package experiment_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MaxBernstetter/dagobert/src/experimentapi"
)

// createViaAPI posts a manual-list experiment with the given settings fragment and returns the status and body.
func createViaAPI(t *testing.T, srv *httptest.Server, settings string) (int, []byte) {
	t.Helper()
	body := fmt.Sprintf(`{"name":"parallelism","mode":"EXPERIMENT_CREATE_MODE_MANUAL",
	  "settings":{"provider":"binance","base":"BTC","quote":"USDT","fromMs":"0","toMs":"%d","redownloadMissing":false%s},
	  "instances":[{"label":"sma 3/6","maCross":{"fastHr":3,"slowHr":6}}]}`, 240*hour, settings)
	resp, err := http.Post(srv.URL+"/api/v1/experiments", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, raw
}

func parallelismServer(t *testing.T) *httptest.Server {
	t.Helper()
	url := pgScratchDB(t)
	pool, err := pgxpool.New(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	mux := http.NewServeMux()
	experimentapi.Mount(mux, experimentapi.NewServer(pool, experimentapi.Hooks{}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestCreateExperimentParallelism(t *testing.T) {
	srv := parallelismServer(t)

	// Set: persisted, returned by create and by GET.
	code, raw := createViaAPI(t, srv, `,"parallelism":3`)
	if code != http.StatusCreated {
		t.Fatalf("create with parallelism 3: %d %s", code, raw)
	}
	var created map[string]any
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatal(err)
	}
	if created["parallelism"] != float64(3) {
		t.Fatalf("create response parallelism: %v", created["parallelism"])
	}
	id, _ := created["id"].(string)
	resp, err := http.Get(srv.URL + "/api/v1/experiments/" + id)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var fetched map[string]any
	if err := json.Unmarshal(got, &fetched); err != nil || fetched["parallelism"] != float64(3) {
		t.Fatalf("GET parallelism: %v %s", err, got)
	}

	// Unset: the key is absent, as before this field existed.
	code, raw = createViaAPI(t, srv, "")
	if code != http.StatusCreated {
		t.Fatalf("create without parallelism: %d %s", code, raw)
	}
	if strings.Contains(string(raw), `"parallelism":`) {
		t.Fatalf("unset parallelism must not appear in the response: %s", raw)
	}

	// Invalid values are rejected with a message.
	if code, raw := createViaAPI(t, srv, `,"parallelism":-1`); code != http.StatusBadRequest || !strings.Contains(string(raw), "must be 0 (auto)") {
		t.Fatalf("negative: %d %s", code, raw)
	}
	above := runtime.NumCPU() + 1
	if code, raw := createViaAPI(t, srv, fmt.Sprintf(`,"parallelism":%d`, above)); code != http.StatusBadRequest || !strings.Contains(string(raw), "above the maximum") {
		t.Fatalf("above the maximum: %d %s", code, raw)
	}

	// The limit the create form shows.
	resp, err = http.Get(srv.URL + "/api/v1/experiments/limits")
	if err != nil {
		t.Fatal(err)
	}
	lim, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(lim) != fmt.Sprintf(`{"maxParallelism":%d}`, runtime.NumCPU()) {
		t.Fatalf("limits: %d %s", resp.StatusCode, lim)
	}
}
