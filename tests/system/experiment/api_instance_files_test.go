package experiment_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"

	"github.com/MaxBernstetter/dagobert/src/backtest"
	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	"github.com/MaxBernstetter/dagobert/src/experimentapi"
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
)

// apiFixture runs two ma-cross instances on the crossing fixture, mirrors the experiment into a
// scratch Postgres and serves the Experiment API over httptest. The second instance's Parquet files
// are removed afterwards, so it stands for an instance without them.
type apiFixture struct {
	srv        *httptest.Server
	dataDir    string
	rec        pgRun
	traded     pgInst
	missing    pgInst
	endSec     int64
	experiment string
}

// newAPIFixture runs two ma-cross instances on the crossing fixture in a scratch Postgres and serves
// the Experiment API over httptest. The second instance's Parquet files are removed afterwards, so
// it stands for an instance without them.
func newAPIFixture(t *testing.T, engine string) apiFixture {
	t.Helper()
	url := pgScratchDB(t)
	dataDir := tempDataDir(t)
	seedLiveMarker(t, dataDir)
	endMs := 240 * hour
	writeCrossingParquet(t, dataDir, symbol, endMs)

	settings := store.DefaultExperimentSettings()
	settings.Base, settings.Quote = "BTC", "USDT"
	settings.FromMs, settings.ToMs = 0, endMs
	settings.RedownloadMissing = false
	rec, err := createExperimentPG(url, "api-files", settings, []fixtureSpec{
		{Label: "traded", Playbook: "ma-cross", Config: json.RawMessage(`{"fastHr":3,"slowHr":6}`)},
		{Label: "missing", Playbook: "ma-cross", Config: json.RawMessage(`{"fastHr":6,"slowHr":12}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("DATABASE_URL", url)
	t.Setenv("VENUE_SIMULATE", "1")
	t.Setenv("DATABASE_URL", url)
	if engine != "" {
		t.Setenv("EXPERIMENT_ENGINE", engine)
		t.Setenv("CPP_SMA_CROSS_BIN", cppSmaCrossBin(t))
	}
	if err := backtest.RunExperiment(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}
	got := loadExperimentPG(t, url, rec.ID)
	for _, inst := range got.Instances {
		if inst.Label == "missing" {
			removeInstanceParquet(t, instanceDir(dataDir, inst))
		}
	}

	pool, err := pgxpool.New(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	mux := http.NewServeMux()
	experimentapi.Mount(mux, experimentapi.NewServer(pool, experimentapi.Hooks{}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	f := apiFixture{srv: srv, dataDir: dataDir, rec: got, endSec: endMs / 1000, experiment: rec.ID}
	for _, inst := range got.Instances {
		switch inst.Label {
		case "traded":
			f.traded = inst
		case "missing":
			f.missing = inst
		}
	}
	return f
}

func (f apiFixture) get(t *testing.T, path string) (int, []byte) {
	t.Helper()
	resp, err := http.Get(f.srv.URL + "/api/v1/experiments/" + f.experiment + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, body
}

func (f apiFixture) instancePath(inst pgInst, suffix string) string {
	return fmt.Sprintf("/instances/%s/%s", inst.ID, suffix)
}

// checkActivity decodes the activity JSON, checks the array shapes and the row keys, and that rows
// are newest candle first. wantRows false asserts both arrays are empty.
func checkActivity(t *testing.T, body []byte, wantRows bool) (rows, journals []map[string]any) {
	t.Helper()
	var resp struct {
		Rows     []map[string]any `json:"rows"`
		Journals []map[string]any `json:"journals"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Rows == nil || resp.Journals == nil {
		t.Fatalf("rows and journals must be arrays, got %s", body)
	}
	if !wantRows {
		if len(resp.Rows) != 0 || len(resp.Journals) != 0 {
			t.Fatalf("expected no rows, got %d rows %d journals", len(resp.Rows), len(resp.Journals))
		}
		return resp.Rows, resp.Journals
	}
	if len(resp.Rows) == 0 || len(resp.Journals) == 0 {
		t.Fatalf("expected rows and journals, got %d rows %d journals", len(resp.Rows), len(resp.Journals))
	}
	for _, key := range []string{"orderLinkId", "playbook", "symbol", "candleClose", "kind", "decision", "side", "cumExecQty", "avgPrice"} {
		if _, ok := resp.Rows[0][key]; !ok {
			t.Fatalf("activity row lacks %q: %v", key, resp.Rows[0])
		}
	}
	prev := int64(1 << 62)
	for _, r := range resp.Rows {
		c := int64(r["candleClose"].(float64))
		if c > prev {
			t.Fatal("activity rows must be newest candle first")
		}
		prev = c
	}
	for _, key := range []string{"orderLinkId", "venue", "venueSymbol", "side", "baseQty", "quoteQty", "price", "fee", "feeCurrency", "priceSource"} {
		if _, ok := resp.Journals[0][key]; !ok {
			t.Fatalf("journal row lacks %q: %v", key, resp.Journals[0])
		}
	}
	return resp.Rows, resp.Journals
}

func TestExperimentAPIInstanceFiles(t *testing.T) {
	f := newAPIFixture(t, "")

	code, body := f.get(t, f.instancePath(f.traded, "activity"))
	if code != http.StatusOK {
		t.Fatalf("activity %d: %s", code, body)
	}
	checkActivity(t, body, true)

	code, body = f.get(t, f.instancePath(f.traded, fmt.Sprintf("chart-series?from=0&to=%d", f.endSec)))
	if code != http.StatusOK {
		t.Fatalf("chart-series %d", code)
	}
	series := &v1.ChartSeries{}
	if err := proto.Unmarshal(body, series); err != nil {
		t.Fatal(err)
	}
	if len(series.Portfolio) < 2 || len(series.Markers) == 0 {
		t.Fatalf("traded instance: portfolio %d markers %d", len(series.Portfolio), len(series.Markers))
	}
	if series.Portfolio[0].TimeSec >= series.Portfolio[1].TimeSec {
		t.Fatal("portfolio must start before the first fill")
	}

	code, body = f.get(t, f.instancePath(f.traded, "kpis"))
	if code != http.StatusOK {
		t.Fatalf("kpis %d: %s", code, body)
	}
	var kpis struct {
		Results []json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(body, &kpis); err != nil || len(kpis.Results) == 0 {
		t.Fatalf("kpis shape: %v %s", err, body)
	}

	// An instance without its Parquet files: empty arrays, still 200.
	code, body = f.get(t, f.instancePath(f.missing, "activity"))
	if code != http.StatusOK {
		t.Fatalf("missing activity %d: %s", code, body)
	}
	checkActivity(t, body, false)

	code, body = f.get(t, f.instancePath(f.missing, fmt.Sprintf("chart-series?from=0&to=%d", f.endSec)))
	if code != http.StatusOK {
		t.Fatalf("missing chart-series %d", code)
	}
	empty := &v1.ChartSeries{}
	if err := proto.Unmarshal(body, empty); err != nil {
		t.Fatal(err)
	}
	if len(empty.Portfolio) != 0 || len(empty.Markers) != 0 {
		t.Fatalf("missing instance: portfolio %d markers %d", len(empty.Portfolio), len(empty.Markers))
	}

	code, _ = f.get(t, f.instancePath(f.missing, "kpis"))
	if code != http.StatusOK {
		t.Fatalf("missing kpis %d", code)
	}
}

// TestExperimentAPIInstanceFilesCpp: C++ instances have real activity rows, journals and markers,
// which they had no source for before the Parquet files (they used to return empty arrays).
func TestExperimentAPIInstanceFilesCpp(t *testing.T) {
	f := newAPIFixture(t, "cpp")
	code, body := f.get(t, f.instancePath(f.traded, "activity"))
	if code != http.StatusOK {
		t.Fatalf("activity %d: %s", code, body)
	}
	checkActivity(t, body, true)

	code, body = f.get(t, f.instancePath(f.traded, fmt.Sprintf("chart-series?from=0&to=%d", f.endSec)))
	if code != http.StatusOK {
		t.Fatalf("chart-series %d", code)
	}
	series := &v1.ChartSeries{}
	if err := proto.Unmarshal(body, series); err != nil {
		t.Fatal(err)
	}
	if len(series.Markers) == 0 {
		t.Fatal("cpp instance returned no trade markers")
	}
}

// removeInstanceParquet deletes the four instance files, as the orchestrator's delete does.
func removeInstanceParquet(t *testing.T, ws string) {
	t.Helper()
	for _, name := range []string{"fills.parquet", "orders.parquet", "journals.parquet", "trade_outcomes.parquet"} {
		if err := os.Remove(filepath.Join(ws, name)); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
}

// TestExperimentPerformanceStableOnRefinalize: /performance is rebuilt from trade_outcomes.parquet on
// every finalize, so a second finalize over the same files gives the same numbers.
func TestExperimentPerformanceStableOnRefinalize(t *testing.T) {
	url := pgScratchDB(t)
	dataDir := tempDataDir(t)
	seedLiveMarker(t, dataDir)
	endMs := 240 * hour
	writeCrossingParquet(t, dataDir, symbol, endMs)
	settings := store.DefaultExperimentSettings()
	settings.Base, settings.Quote = "BTC", "USDT"
	settings.FromMs, settings.ToMs = 0, endMs
	settings.RedownloadMissing = false
	rec, err := createExperimentPG(url, "api-performance", settings, []fixtureSpec{
		{Label: "a", Playbook: "ma-cross", Config: json.RawMessage(`{"fastHr":3,"slowHr":6}`)},
		{Label: "b", Playbook: "ma-cross", Config: json.RawMessage(`{"fastHr":6,"slowHr":12}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("DATABASE_URL", url)
	t.Setenv("VENUE_SIMULATE", "1")
	t.Setenv("DATABASE_URL", url)
	if err := backtest.RunExperiment(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}

	pool, err := pgxpool.New(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	mux := http.NewServeMux()
	experimentapi.Mount(mux, experimentapi.NewServer(pool, experimentapi.Hooks{}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	f := apiFixture{srv: srv, dataDir: dataDir, experiment: rec.ID}

	code, first := f.get(t, "/performance")
	if code != http.StatusOK {
		t.Fatalf("performance %d: %s", code, first)
	}
	if err := backtest.RunExperimentFinalize(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}
	code, second := f.get(t, "/performance")
	if code != http.StatusOK {
		t.Fatalf("performance after finalize %d: %s", code, second)
	}
	if string(first) != string(second) {
		t.Fatalf("performance changed on refinalize:\n%s\n%s", first, second)
	}
	if !strings.Contains(string(first), `"results":[{`) {
		t.Fatalf("performance has no results: %s", first)
	}
}

// TestExperimentAPIDeleteRemovesFiles: DELETE drops the experiment's directory under DATA_DIR.
func TestExperimentAPIDeleteRemovesFiles(t *testing.T) {
	f := newAPIFixture(t, "")
	dir := filepath.Join(f.dataDir, "experiments", f.experiment)
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("experiment directory before delete: %v", err)
	}
	req, err := http.NewRequest(http.MethodDelete, f.srv.URL+"/api/v1/experiments/"+f.experiment, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete status %d", resp.StatusCode)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("experiment directory still exists after delete: %v", err)
	}
}
