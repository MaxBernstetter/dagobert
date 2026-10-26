package candlefile

import (
	"os"
	"testing"

	"github.com/parquet-go/parquet-go"

	"github.com/MaxBernstetter/dagobert/src/internal/parquetmeta"
)

func TestMigrateFileIsAtomicIdempotentAndExact(t *testing.T) {
	path := t.TempDir() + "/candles.parquet"
	old := []legacyRow{
		{1, "100.25", "101.5", "99.75", "100.5", "12.345", 2},
		{3, "82409.43000000", "1", "1", "1", "1069.70305000", 4},
	}
	if err := parquet.WriteFile(path, old,
		parquetmeta.WriterOptions(parquetmeta.Meta{SchemaVersion: 1, IntervalMS: 3600000})...); err != nil {
		t.Fatal(err)
	}
	did, n, err := MigrateFile(path)
	if err != nil || !did || n != 2 {
		t.Fatalf("first run: did=%v n=%d err=%v", did, n, err)
	}
	did, _, err = MigrateFile(path)
	if err != nil || did {
		t.Fatalf("second run must skip: did=%v err=%v", did, err)
	}
	m, ok, err := parquetmeta.Read(path)
	if err != nil || !ok || m.IntervalMS != 3600000 || m.SchemaVersion != SchemaVersion {
		t.Fatalf("meta=%+v ok=%v err=%v", m, ok, err)
	}
	// A value the layout cannot hold leaves the original untouched.
	bad := t.TempDir() + "/candles.parquet"
	if err := parquet.WriteFile(bad, []legacyRow{{1, "1.123456789", "1", "1", "1", "1", 2}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := MigrateFile(bad); err == nil {
		t.Fatal("want error for 9 decimals")
	}
	pf := openFile(t, bad)
	if !IsLegacy(pf) {
		t.Fatal("failed migration must keep the original")
	}
}

func openFile(t *testing.T, path string) *parquet.File {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return open(t, b)
}
