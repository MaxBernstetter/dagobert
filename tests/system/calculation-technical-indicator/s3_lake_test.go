package process

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/MaxBernstetter/dagobert/src/shared"
	"github.com/MaxBernstetter/dagobert/tests/system/internal/floci"
)

// With LAKE_BACKEND=s3 the job reads candles from the bucket and stores its indicator bars there, with
// nothing left on the local disk.
func TestIndicatorBarsAreStoredInTheS3Lake(t *testing.T) {
	bucket := floci.BucketName("lake-test")
	endpoint, client := floci.Start(t, bucket)
	ctx := context.Background()

	dir := dataDir(t)
	writeCandles(t, dir, "BTCUSDT", case1Candles())
	candlePath, err := shared.ParquetPath(dir, "binance", "BTC", "USDT", 1970)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(candlePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String("parquet/provider=binance/base=BTC/quote=USDT/year=1970/candles.parquet"),
		Body:   bytes.NewReader(body),
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(candlePath); err != nil {
		t.Fatal(err)
	}

	env := map[string]string{
		"LAKE_BACKEND":          "s3",
		"LAKE_BUCKET":           bucket,
		"AWS_ENDPOINT_URL":      endpoint,
		"AWS_ACCESS_KEY_ID":     "test",
		"AWS_SECRET_ACCESS_KEY": "test",
		"AWS_REGION":            "us-east-1",
	}
	stdout, stderr, code := runProcessWith(t, dir, threeHours, env, "BTCUSDT", "0", strconv.FormatInt(threeHours, 10))
	if code != 0 || !hasDetail(stdout, 0) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	key := "parquet/ti/provider=binance/base=BTC/quote=USDT/algo=v1/year=1970/bars.parquet"
	obj, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		t.Fatalf("indicator bars missing from the lake at %s: %v", key, err)
	}
	n, _ := io.Copy(io.Discard, obj.Body)
	obj.Body.Close()
	if n == 0 {
		t.Fatal("empty indicator bars object")
	}
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(p, ".parquet") {
			t.Errorf("local parquet left behind: %s", p)
		}
		return nil
	})
}
