package parquetmeta

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/parquet-go/parquet-go"

	"github.com/MaxBernstetter/dagobert/src/shared"
)

type row struct {
	OpenTime int64 `parquet:"open_time"`
}

func TestWriteReadFooterMeta(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "candles.parquet")
	meta := Meta{IntervalMS: 3_600_000, DataSource: shared.DataSourceBinance}
	if err := parquet.WriteFile(path, []row{{OpenTime: 1}}, WriterOptions(meta)...); err != nil {
		t.Fatal(err)
	}
	got, ok, err := Read(path)
	if err != nil || !ok {
		t.Fatalf("Read: ok=%v err=%v", ok, err)
	}
	if got.SchemaVersion != SchemaVersion {
		t.Fatalf("schema_version: %d", got.SchemaVersion)
	}
	if got.IntervalMS != meta.IntervalMS {
		t.Fatalf("interval_ms: %d", got.IntervalMS)
	}
	if got.DataSource != meta.DataSource {
		t.Fatalf("data_source: %v", got.DataSource)
	}
	if _, err := os.Stat(path + ".meta.json"); !os.IsNotExist(err) {
		t.Fatalf("sidecar should not exist: %v", err)
	}
}
