package experiment_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/parquet-go/parquet-go"
	"google.golang.org/protobuf/proto"

	"github.com/MaxBernstetter/dagobert/src/backtest"
	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	"github.com/MaxBernstetter/dagobert/src/experimentapi"
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
)

// apiFixture runs two ma-cross instances on the crossing fixture, mirrors the experiment into a
// scratch Postgres and serves the Experiment API over httptest. The second instance is moved into a
// shard of its own that holds its report but no rows, so it stands for an instance without them.
type apiFixture struct {
	srv        *httptest.Server
	pool       *pgxpool.Pool
	dataDir    string
	rec        pgRun
	traded     pgInst
	missing    pgInst
	endSec     int64
	experiment string
}

// newAPIFixture runs two ma-cross instances on the crossing fixture in a scratch Postgres and serves
// the Experiment API over httptest. The second instance is moved into a shard of its own that holds
// its report but no rows, so it stands for an instance without them.
func newAPIFixture(t *testing.T) apiFixture {
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
	if err := backtest.RunExperiment(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}
	got := loadExperimentPG(t, url, rec.ID)
	pool, err := pgxpool.New(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for _, inst := range got.Instances {
		if inst.Label == "missing" {
			moveToReportOnlyShard(t, pool, dataDir, rec.ID, inst)
		}
	}
	mux := http.NewServeMux()
	experimentapi.Mount(mux, experimentapi.NewServer(pool, experimentapi.Hooks{}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	f := apiFixture{srv: srv, pool: pool, dataDir: dataDir, rec: got, endSec: endMs / 1000, experiment: rec.ID}
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
	f := newAPIFixture(t)

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

	// An instance without rows in its shard: empty arrays, still 200.
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

// moveToReportOnlyShard points the instance at a new shard that holds its report and no rows.
func moveToReportOnlyShard(t *testing.T, pool *pgxpool.Pool, dataDir, experimentID string, inst pgInst) {
	t.Helper()
	report := readReport(t, instanceRef(dataDir, inst))
	key := "experiments/" + experimentID + "/shards/report-only"
	dir := filepath.Join(dataDir, key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, "reports.parquet"))
	if err != nil {
		t.Fatal(err)
	}
	w := parquet.NewGenericWriter[reportFile](f, parquet.KeyValueMetadata("trading_copilot.instances", fmt.Sprintf(`{%q:0}`, inst.ID)))
	if _, err := w.Write([]reportFile{{InstanceID: inst.ID, ReportJSON: string(report)}}); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), `UPDATE experiment_instance SET shard_key=$2 WHERE id=$1`, inst.ID, key); err != nil {
		t.Fatal(err)
	}
}

// TestExperimentAPIInstanceWithoutShardKey: an instance without shard_key (not completed, or run
// before shards) reads like a missing file: no report or KPIs (404), empty activity and chart (200).
func TestExperimentAPIInstanceWithoutShardKey(t *testing.T) {
	f := newAPIFixture(t)
	if _, err := f.pool.Exec(t.Context(), `UPDATE experiment_instance SET shard_key='' WHERE id=$1`, f.traded.ID); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"report", "kpis"} {
		if code, body := f.get(t, f.instancePath(f.traded, suffix)); code != http.StatusNotFound {
			t.Fatalf("%s: status %d want 404: %s", suffix, code, body)
		}
	}
	code, body := f.get(t, f.instancePath(f.traded, "activity"))
	if code != http.StatusOK {
		t.Fatalf("activity %d: %s", code, body)
	}
	checkActivity(t, body, false)
	code, body = f.get(t, f.instancePath(f.traded, fmt.Sprintf("chart-series?from=0&to=%d", f.endSec)))
	if code != http.StatusOK {
		t.Fatalf("chart-series %d", code)
	}
	series := &v1.ChartSeries{}
	if err := proto.Unmarshal(body, series); err != nil {
		t.Fatal(err)
	}
	if len(series.Portfolio) != 0 || len(series.Markers) != 0 {
		t.Fatalf("portfolio %d markers %d", len(series.Portfolio), len(series.Markers))
	}
}

// TestExperimentPerformanceFromParquet: /performance is built from the instances' trade outcome row groups
// at finalize, and the prepare cache is gone once the experiment is final.
func TestExperimentPerformanceFromParquet(t *testing.T) {
	f := newAPIFixture(t)
	code, body := f.get(t, "/performance")
	if code != http.StatusOK {
		t.Fatalf("performance %d: %s", code, body)
	}
	if !strings.Contains(string(body), `"results":[{`) {
		t.Fatalf("performance has no results: %s", body)
	}
	if _, err := os.Stat(filepath.Join(f.dataDir, "experiments", f.experiment, ".cache")); !os.IsNotExist(err) {
		t.Fatalf("prepare cache still present after finalize: %v", err)
	}
}

// TestExperimentAPIDeleteRemovesFiles: DELETE drops the experiment's directory under DATA_DIR.
func TestExperimentAPIDeleteRemovesFiles(t *testing.T) {
	f := newAPIFixture(t)
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

// TestExperimentRunNeedsDatabase: the experiment catalog is Postgres; a run without DATABASE_URL fails
// before it touches any instance.
func TestExperimentRunNeedsDatabase(t *testing.T) {
	url := pgScratchDB(t)
	dataDir := tempDataDir(t)
	seedLiveMarker(t, dataDir)
	writeCrossingParquet(t, dataDir, symbol, 240*hour)
	settings := store.DefaultExperimentSettings()
	settings.Base, settings.Quote = "BTC", "USDT"
	settings.FromMs, settings.ToMs = 0, 240*hour
	rec, err := createExperimentPG(url, "no-db", settings, []fixtureSpec{
		{Label: "a", Playbook: "ma-cross", Config: json.RawMessage(`{"fastHr":3,"slowHr":6}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("DATABASE_URL", "")
	if err := backtest.RunExperiment(dataDir, rec.ID); err == nil {
		t.Fatal("run without DATABASE_URL must fail")
	}
}

// TestChartSeriesMaxPoints: without maxPoints (or with 0) the series is unchanged. With maxPoints the
// candles are aggregated to at most that many, and the portfolio and the markers stay exact.
func TestChartSeriesMaxPoints(t *testing.T) {
	f := newAPIFixture(t)
	base := fmt.Sprintf("/instances/%s/chart-series?from=0&to=%d", f.traded.ID, f.endSec)

	code, plain := f.get(t, base)
	if code != http.StatusOK {
		t.Fatalf("chart-series %d", code)
	}
	code, zero := f.get(t, base+"&maxPoints=0")
	if code != http.StatusOK || string(zero) != string(plain) {
		t.Fatalf("maxPoints=0 must be byte-identical to no parameter (status %d)", code)
	}
	full := &v1.ChartSeries{}
	if err := proto.Unmarshal(plain, full); err != nil {
		t.Fatal(err)
	}
	if len(full.TimeSec) < 20 {
		t.Fatalf("fixture has only %d candles, need more than the budget", len(full.TimeSec))
	}

	const budget = 10
	code, body := f.get(t, base+fmt.Sprintf("&maxPoints=%d", budget))
	if code != http.StatusOK {
		t.Fatalf("maxPoints chart-series %d", code)
	}
	small := &v1.ChartSeries{}
	if err := proto.Unmarshal(body, small); err != nil {
		t.Fatal(err)
	}
	if len(small.TimeSec) == 0 || len(small.TimeSec) > budget {
		t.Fatalf("candles %d, budget %d", len(small.TimeSec), budget)
	}
	for _, ind := range small.Indicators {
		if len(ind.Value) != len(small.TimeSec) {
			t.Fatalf("indicator %s has %d values for %d candles", ind.Id, len(ind.Value), len(small.TimeSec))
		}
	}
	if small.TimeSec[0] != full.TimeSec[0] || small.Open[0] != full.Open[0] {
		t.Fatal("first bucket must start at the first candle")
	}
	if small.Close[len(small.Close)-1] != full.Close[len(full.Close)-1] {
		t.Fatal("last bucket must close at the last candle's close")
	}
	if !reflect.DeepEqual(small.Portfolio, full.Portfolio) {
		t.Fatal("portfolio must not be downsampled")
	}
	if !reflect.DeepEqual(small.Markers, full.Markers) {
		t.Fatal("markers must not be downsampled")
	}
	if len(full.Markers) == 0 {
		t.Fatal("fixture must have markers")
	}
}
