package experiment_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/parquet-go/parquet-go"
	"google.golang.org/protobuf/proto"

	"github.com/MaxBernstetter/dagobert/src/backtest"
	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	"github.com/MaxBernstetter/dagobert/src/experimentapi"
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1/tradingcopilotv1connect"
)

// apiFixture runs two ma-cross instances on the crossing fixture, mirrors the experiment into a
// scratch Postgres and serves ExperimentService over httptest. The second instance is moved into a
// shard of its own that holds its report but no rows, so it stands for an instance without them.
type apiFixture struct {
	srv        *httptest.Server
	client     tradingcopilotv1connect.ExperimentServiceClient
	pool       *pgxpool.Pool
	dataDir    string
	rec        pgRun
	traded     pgInst
	missing    pgInst
	endSec     int64
	experiment string
}

// newAPIFixture runs two ma-cross instances on the crossing fixture in a scratch Postgres and serves
// ExperimentService over httptest. The second instance is moved into a shard of its own that holds
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
	srv := serveExperimentAPI(t, pool)

	f := apiFixture{srv: srv, client: newExperimentClient(srv), pool: pool, dataDir: dataDir, rec: got, endSec: endMs / 1000, experiment: rec.ID}
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

// serveExperimentAPI mounts ExperimentService on an httptest server.
func serveExperimentAPI(t *testing.T, pool *pgxpool.Pool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	experimentapi.Mount(mux, experimentapi.NewServer(pool, experimentapi.Hooks{}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newExperimentClient(srv *httptest.Server) tradingcopilotv1connect.ExperimentServiceClient {
	return tradingcopilotv1connect.NewExperimentServiceClient(srv.Client(), srv.URL)
}

func (f apiFixture) activity(t *testing.T, inst pgInst) *v1.GetInstanceActivityResponse {
	t.Helper()
	resp, err := f.client.GetInstanceActivity(t.Context(), connect.NewRequest(&v1.GetInstanceActivityRequest{ExperimentId: f.experiment, InstanceId: inst.ID}))
	if err != nil {
		t.Fatalf("activity: %v", err)
	}
	return resp.Msg
}

// chart asks for the whole window of the fixture; maxPoints 0 keeps every candle.
func (f apiFixture) chart(t *testing.T, inst pgInst, maxPoints int32) *v1.ChartSeries {
	t.Helper()
	resp, err := f.client.GetChartSeries(t.Context(), connect.NewRequest(&v1.GetChartSeriesRequest{
		ExperimentId: f.experiment, InstanceId: inst.ID, FromSec: 0, ToSec: f.endSec, MaxPoints: maxPoints,
	}))
	if err != nil {
		t.Fatalf("chart-series: %v", err)
	}
	return resp.Msg.Series
}

func (f apiFixture) kpis(inst pgInst) (*connect.Response[v1.GetInstanceKpisResponse], error) {
	return f.client.GetInstanceKpis(context.Background(), connect.NewRequest(&v1.GetInstanceKpisRequest{ExperimentId: f.experiment, InstanceId: inst.ID}))
}

func (f apiFixture) report(inst pgInst) (*connect.Response[v1.GetInstanceReportResponse], error) {
	return f.client.GetInstanceReport(context.Background(), connect.NewRequest(&v1.GetInstanceReportRequest{ExperimentId: f.experiment, InstanceId: inst.ID}))
}

// checkActivity checks the activity rows: the fields a fill carries and the order, newest candle first.
// wantRows false asserts both lists are empty.
func checkActivity(t *testing.T, resp *v1.GetInstanceActivityResponse, wantRows bool) {
	t.Helper()
	if !wantRows {
		if len(resp.Rows) != 0 || len(resp.Journals) != 0 {
			t.Fatalf("expected no rows, got %d rows %d journals", len(resp.Rows), len(resp.Journals))
		}
		return
	}
	if len(resp.Rows) == 0 || len(resp.Journals) == 0 {
		t.Fatalf("expected rows and journals, got %d rows %d journals", len(resp.Rows), len(resp.Journals))
	}
	first := resp.Rows[0]
	if first.OrderLinkId == "" || first.Playbook == "" || first.Symbol == "" || first.Kind == "" {
		t.Fatalf("activity row lacks identity fields: %v", first)
	}
	if first.Decision == nil || first.Side == nil || first.CumExecQty == nil || first.AvgPrice == nil {
		t.Fatalf("activity row lacks fill fields: %v", first)
	}
	prev := int64(1 << 62)
	for _, r := range resp.Rows {
		if r.CandleClose > prev {
			t.Fatal("activity rows must be newest candle first")
		}
		prev = r.CandleClose
	}
	if j := resp.Journals[0]; j.OrderLinkId == "" || j.Venue == "" || j.Price == "" {
		t.Fatalf("journal row lacks fields: %v", j)
	}
}

func TestExperimentAPIInstanceFiles(t *testing.T) {
	f := newAPIFixture(t)

	checkActivity(t, f.activity(t, f.traded), true)

	series := f.chart(t, f.traded, 0)
	if len(series.Portfolio) < 2 || len(series.Markers) == 0 {
		t.Fatalf("traded instance: portfolio %d markers %d", len(series.Portfolio), len(series.Markers))
	}
	if series.Portfolio[0].TimeSec >= series.Portfolio[1].TimeSec {
		t.Fatal("portfolio must start before the first fill")
	}

	kpis, err := f.kpis(f.traded)
	if err != nil || len(kpis.Msg.Kpis.Results) == 0 {
		t.Fatalf("kpis shape: %v %v", err, kpis)
	}

	// An instance without rows in its shard: empty lists, still a result.
	checkActivity(t, f.activity(t, f.missing), false)

	empty := f.chart(t, f.missing, 0)
	if len(empty.Portfolio) != 0 || len(empty.Markers) != 0 {
		t.Fatalf("missing instance: portfolio %d markers %d", len(empty.Portfolio), len(empty.Markers))
	}

	if _, err := f.kpis(f.missing); err != nil {
		t.Fatalf("missing kpis: %v", err)
	}
}

// TestExperimentAPICodes: an unknown instance is CodeNotFound, a malformed id CodeInvalidArgument.
func TestExperimentAPICodes(t *testing.T) {
	f := newAPIFixture(t)
	unknown := pgInst{ID: "00000000-0000-0000-0000-000000000000"}
	if _, err := f.kpis(unknown); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("unknown instance: %v", err)
	}
	if _, err := f.client.GetInstance(t.Context(), connect.NewRequest(&v1.GetInstanceRequest{ExperimentId: f.experiment, InstanceId: "nope"})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("malformed instance id: %v", err)
	}
	if _, err := f.client.GetExperiment(t.Context(), connect.NewRequest(&v1.GetExperimentRequest{Id: unknown.ID})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("unknown experiment: %v", err)
	}
}

// TestExperimentAPIJSON: the same method answers a plain JSON POST, as curl or an API gateway sends it.
func TestExperimentAPIJSON(t *testing.T) {
	f := newAPIFixture(t)
	post := func(method, body string) (int, []byte) {
		resp, err := http.Post(f.srv.URL+"/tradingcopilot.v1.ExperimentService/"+method, "application/json", bytes.NewReader([]byte(body)))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, raw
	}
	code, raw := post("GetInstanceKpis", fmt.Sprintf(`{"experimentId":%q,"instanceId":%q}`, f.experiment, f.traded.ID))
	var kpis struct {
		Kpis struct {
			Results []json.RawMessage `json:"results"`
		} `json:"kpis"`
	}
	if err := json.Unmarshal(raw, &kpis); code != http.StatusOK || err != nil || len(kpis.Kpis.Results) == 0 {
		t.Fatalf("kpis over JSON: %d %v %s", code, err, raw)
	}
	code, raw = post("GetInstanceKpis", fmt.Sprintf(`{"experimentId":%q,"instanceId":"00000000-0000-0000-0000-000000000000"}`, f.experiment))
	var failure struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(raw, &failure); code != http.StatusNotFound || err != nil || failure.Code != "not_found" {
		t.Fatalf("unknown instance over JSON: %d %v %s", code, err, raw)
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
// before shards) reads like a missing file: no report or KPIs (CodeNotFound), empty activity and chart.
func TestExperimentAPIInstanceWithoutShardKey(t *testing.T) {
	f := newAPIFixture(t)
	if _, err := f.pool.Exec(t.Context(), `UPDATE experiment_instance SET shard_key='' WHERE id=$1`, f.traded.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.report(f.traded); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("report: %v want CodeNotFound", err)
	}
	if _, err := f.kpis(f.traded); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("kpis: %v want CodeNotFound", err)
	}
	checkActivity(t, f.activity(t, f.traded), false)
	series := f.chart(t, f.traded, 0)
	if len(series.Portfolio) != 0 || len(series.Markers) != 0 {
		t.Fatalf("portfolio %d markers %d", len(series.Portfolio), len(series.Markers))
	}
}

// TestExperimentPerformanceFromParquet: GetExperimentPerformance is built from the instances' trade outcome row groups
// at finalize, and the prepare cache is gone once the experiment is final.
func TestExperimentPerformanceFromParquet(t *testing.T) {
	f := newAPIFixture(t)
	perf, err := f.client.GetExperimentPerformance(t.Context(), connect.NewRequest(&v1.GetExperimentPerformanceRequest{Id: f.experiment}))
	if err != nil {
		t.Fatalf("performance: %v", err)
	}
	if len(perf.Msg.Performance.Results) == 0 {
		t.Fatalf("performance has no results: %v", perf.Msg)
	}
	if _, err := os.Stat(filepath.Join(f.dataDir, "experiments", f.experiment, ".cache")); !os.IsNotExist(err) {
		t.Fatalf("prepare cache still present after finalize: %v", err)
	}
}

// TestExperimentAPIDeleteRemovesFiles: DeleteExperiment drops the experiment's directory under DATA_DIR.
func TestExperimentAPIDeleteRemovesFiles(t *testing.T) {
	f := newAPIFixture(t)
	dir := filepath.Join(f.dataDir, "experiments", f.experiment)
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("experiment directory before delete: %v", err)
	}
	if _, err := f.client.DeleteExperiment(t.Context(), connect.NewRequest(&v1.DeleteExperimentRequest{Id: f.experiment})); err != nil {
		t.Fatalf("delete: %v", err)
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

	plain := f.chart(t, f.traded, 0)
	full := f.chart(t, f.traded, -1)
	if !proto.Equal(plain, full) {
		t.Fatal("a non-positive maxPoints must leave the series unchanged")
	}
	if len(full.TimeSec) < 20 {
		t.Fatalf("fixture has only %d candles, need more than the budget", len(full.TimeSec))
	}

	const budget = 10
	small := f.chart(t, f.traded, budget)
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
	if !proto.Equal(&v1.ChartSeries{Portfolio: small.Portfolio}, &v1.ChartSeries{Portfolio: full.Portfolio}) {
		t.Fatal("portfolio must not be downsampled")
	}
	if !proto.Equal(&v1.ChartSeries{Markers: small.Markers}, &v1.ChartSeries{Markers: full.Markers}) {
		t.Fatal("markers must not be downsampled")
	}
	if len(full.Markers) == 0 {
		t.Fatal("fixture must have markers")
	}
}
