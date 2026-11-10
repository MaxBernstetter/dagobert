package experiment_test

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MaxBernstetter/dagobert/src/backtest"
	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"github.com/MaxBernstetter/dagobert/tests/system/internal/floci"
)

func bucketKeys(t *testing.T, client *s3.Client, bucket, prefix string) []string {
	t.Helper()
	var keys []string
	p := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{Bucket: aws.String(bucket), Prefix: aws.String(prefix)})
	for p.HasMorePages() {
		page, err := p.NextPage(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range page.Contents {
			keys = append(keys, aws.ToString(o.Key))
		}
	}
	return keys
}

// uploadLake moves the candle Parquet files the fixture wrote under dataDir/parquet into the bucket
// and removes them from the disk, so the run can only read them from S3.
func uploadLake(t *testing.T, client *s3.Client, bucket, dataDir string) {
	t.Helper()
	root := filepath.Join(dataDir, "parquet")
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dataDir, p)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		_, err = client.PutObject(context.Background(), &s3.PutObjectInput{
			Bucket: aws.String(bucket), Key: aws.String(filepath.ToSlash(rel)), Body: strings.NewReader(string(body)),
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
}

// TestExperimentWorkspaceOnS3 runs an experiment with LAKE_BACKEND=s3: shards and the prepare cache
// are objects in the bucket, nothing is left on the local disk, the Experiment API serves the
// instance data from the bucket, and deleting the experiment removes every object.
func TestExperimentWorkspaceOnS3(t *testing.T) {
	const bucket = "workspace-test"
	endpoint, client := floci.Start(t, bucket)

	url := pgScratchDB(t)
	dataDir := tempDataDir(t)
	seedLiveMarker(t, dataDir)
	endMs := 240 * hour
	writeCrossingParquet(t, dataDir, symbol, endMs)
	uploadLake(t, client, bucket, dataDir)

	settings := store.DefaultExperimentSettings()
	settings.Base, settings.Quote = "BTC", "USDT"
	settings.FromMs, settings.ToMs = 0, endMs
	settings.RedownloadMissing = false
	rec, err := createExperimentPG(url, "s3-workspace", settings, []fixtureSpec{
		{Label: "traded", Playbook: "ma-cross", Config: json.RawMessage(`{"fastHr":3,"slowHr":6}`)},
		{Label: "slow", Playbook: "ma-cross", Config: json.RawMessage(`{"fastHr":6,"slowHr":12}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("DATABASE_URL", url)
	t.Setenv("VENUE_SIMULATE", "1")
	t.Setenv("LAKE_BACKEND", "s3")
	t.Setenv("LAKE_BUCKET", bucket)
	t.Setenv("AWS_ENDPOINT_URL", endpoint)
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("CPP_SMA_CROSS_BIN", cppSmaCrossBin(t))
	prefix := "experiments/" + rec.ID + "/"

	// Prepare hands over through the bucket: meta, instance list and the KPI series per symbol.
	if err := backtest.RunExperimentPrepare(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}
	cache := bucketKeys(t, client, bucket, prefix+".cache/")
	for _, want := range []string{"prepare_meta.json", "instances.json", "kpi/"} {
		found := false
		for _, k := range cache {
			found = found || strings.Contains(k, prefix+".cache/"+want)
		}
		if !found {
			t.Fatalf("prepare left no %s object, got %v", want, cache)
		}
	}
	ids, err := backtest.ReadExperimentInstanceList(dataDir, rec.ID)
	if err != nil || len(ids) != 2 {
		t.Fatalf("instance list %v: %v", ids, err)
	}
	if err := backtest.RunExperimentSimulate(dataDir, rec.ID, ids); err != nil {
		t.Fatal(err)
	}
	if err := backtest.RunExperimentFinalize(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}

	got := loadExperimentPG(t, url, rec.ID)
	if got.Status != store.StatusCompleted || countStatus(got)[store.StatusCompleted] != 2 {
		t.Fatalf("status %q error %q counts %v", got.Status, got.Error, countStatus(got))
	}
	keys := bucketKeys(t, client, bucket, prefix)
	shardFiles := 0
	for _, k := range keys {
		if strings.Contains(k, "/.cache/") {
			t.Errorf("prepare cache object left after finalize: %s", k)
		}
		if strings.HasPrefix(k, prefix+"shards/") && strings.HasSuffix(k, ".parquet") {
			shardFiles++
		}
	}
	if shardFiles != 6 {
		t.Fatalf("want 6 shard files (one shard), got %d in %v", shardFiles, keys)
	}
	for _, inst := range got.Instances {
		if !strings.HasPrefix(inst.ShardKey, prefix+"shards/") {
			t.Fatalf("instance %s shard key %q", inst.Label, inst.ShardKey)
		}
	}
	_ = filepath.WalkDir(dataDir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && (strings.HasSuffix(p, ".parquet") || strings.Contains(p, ".cache") || filepath.Base(p) == "paper_venue.json") {
			t.Errorf("workspace file left on the local disk: %s", p)
		}
		return nil
	})

	pool, err := pgxpool.New(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	srv := serveExperimentAPI(t, pool)
	f := apiFixture{srv: srv, client: newExperimentClient(srv), pool: pool, dataDir: dataDir, rec: got, endSec: endMs / 1000, experiment: rec.ID}
	for _, inst := range got.Instances {
		if inst.Label == "traded" {
			f.traded = inst
		}
	}
	if resp, err := f.report(f.traded); err != nil || resp.Msg.Report.Fields["kpis"] == nil {
		t.Fatalf("report: %v %v", err, resp)
	}
	kpis, err := f.kpis(f.traded)
	if err != nil {
		t.Fatalf("kpis: %v", err)
	}
	if !hasKpi(kpis.Msg.Kpis, "win-rate") {
		t.Fatalf("kpis lack win-rate: %v", kpis.Msg.Kpis)
	}
	checkActivity(t, f.activity(t, f.traded), true)
	series := f.chart(t, f.traded, 0)
	if len(series.TimeSec) == 0 || len(series.Portfolio) == 0 || len(series.Markers) == 0 {
		t.Fatalf("chart from the bucket: candles %d portfolio %d markers %d", len(series.TimeSec), len(series.Portfolio), len(series.Markers))
	}
	perf, err := f.client.GetExperimentPerformance(t.Context(), connect.NewRequest(&v1.GetExperimentPerformanceRequest{Id: f.experiment}))
	if err != nil || len(perf.Msg.Performance.Results) == 0 {
		t.Fatalf("performance: %v %v", err, perf)
	}

	if _, err := f.client.DeleteExperiment(t.Context(), connect.NewRequest(&v1.DeleteExperimentRequest{Id: rec.ID})); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if left := bucketKeys(t, client, bucket, prefix); len(left) != 0 {
		t.Fatalf("objects left after delete: %v", left)
	}
}

func hasKpi(report *v1.KpiReport, slug string) bool {
	for _, r := range report.Results {
		if r.Def.Slug == slug {
			return true
		}
	}
	return false
}
