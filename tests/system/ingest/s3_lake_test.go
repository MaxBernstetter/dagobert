package ingest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/parquet-go/parquet-go"
)

// startFloci runs Floci on a free loopback port and returns its S3 endpoint.
func startFloci(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	name := fmt.Sprintf("floci-ingest-test-%d", port)
	out, err := exec.Command("docker", "run", "-d", "--rm", "--name", name,
		"-p", fmt.Sprintf("127.0.0.1:%d:4566", port), "floci/floci:latest").CombinedOutput()
	if err != nil {
		t.Fatalf("start floci: %v\n%s", err, out)
	}
	t.Cleanup(func() { exec.Command("docker", "rm", "-f", name).Run() })
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
	for i := 0; i < 60; i++ {
		if resp, err := http.Get(endpoint + "/_floci/health"); err == nil {
			resp.Body.Close()
			return endpoint
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("floci did not become healthy")
	return ""
}

func TestCandlesAreStoredInTheS3LakeNotOnDisk(t *testing.T) {
	endpoint := startFloci(t)
	client := s3.New(s3.Options{
		Region:       "us-east-1",
		BaseEndpoint: aws.String(endpoint),
		UsePathStyle: true,
		Credentials:  credentials.NewStaticCredentialsProvider("test", "test", ""),
	})
	const bucket = "lake-test"
	if _, err := client.CreateBucket(context.Background(), &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatal(err)
	}

	open0 := hourFloor(1_700_000_000_000)
	open1 := open0 + hourMS
	var hits atomic.Int32
	base := serve(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		writeJSON(t, w, []kline{
			candleRow(open0, "100.0", "110.0", "90.0", "105.0", "1.5"),
			candleRow(open1, "105.0", "120.0", "100.0", "115.0", "2.5"),
		})
	})
	env := map[string]string{
		"LAKE_BACKEND":          "s3",
		"LAKE_BUCKET":           bucket,
		"AWS_ENDPOINT_URL":      endpoint,
		"AWS_ACCESS_KEY_ID":     "test",
		"AWS_SECRET_ACCESS_KEY": "test",
		"AWS_REGION":            "us-east-1",
	}
	dir := t.TempDir()

	stdout, stderr, code := runIngest(t, dir, base, env, open0, open1+hourMS, "BTCUSDT")
	if code != 0 || !bytes.Contains(stdout, []byte("DETAIL=0\n")) {
		t.Fatalf("first run: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	// The bucket holds the file, at the Hive key.
	key := "parquet/provider=binance/base=BTC/quote=USDT/year=2023/candles.parquet"
	obj, err := client.GetObject(context.Background(), &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		t.Fatalf("object %s missing: %v", key, err)
	}
	body, err := io.ReadAll(obj.Body)
	obj.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(t.TempDir(), "candles.parquet")
	if err := os.WriteFile(local, body, 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := parquet.ReadFile[diskCandle](local)
	if err != nil || len(rows) != 2 {
		t.Fatalf("parquet from the lake: rows=%d err=%v", len(rows), err)
	}

	// No parquet is left on the local disk.
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(p, ".parquet") {
			t.Errorf("local parquet left behind: %s", p)
		}
		return nil
	})

	// A second run finds the hours in the lake and does not call the upstream.
	before := hits.Load()
	stdout, stderr, code = runIngest(t, dir, base, env, open0, open1+hourMS, "BTCUSDT")
	if code != 2 || !bytes.Contains(stdout, []byte("DETAIL=1\n")) {
		t.Fatalf("second run: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if hits.Load() != before {
		t.Fatalf("second run called the upstream: before=%d after=%d", before, hits.Load())
	}
}

func TestUploadFailureFailsTheRun(t *testing.T) {
	open0 := hourFloor(1_700_000_000_000)
	base := serve(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, []kline{candleRow(open0, "100.0", "110.0", "90.0", "105.0", "1.5")})
	})
	// No bucket exists at this endpoint: the lake is the only store, so the run must fail loudly.
	env := map[string]string{
		"LAKE_BACKEND":     "s3",
		"LAKE_BUCKET":      "does-not-exist",
		"AWS_ENDPOINT_URL": "http://127.0.0.1:1",
		"AWS_REGION":       "us-east-1",
	}
	_, _, code := runIngest(t, t.TempDir(), base, env, open0, open0+hourMS, "BTCUSDT")
	if code != 1 {
		t.Fatalf("exit=%d, want 1 (failed)", code)
	}
}
