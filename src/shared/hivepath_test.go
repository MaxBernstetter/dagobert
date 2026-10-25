package shared

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParquetPathHive(t *testing.T) {
	p, err := ParquetPath("/data", "binance", "btc", "usdt", 2024)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/data", "parquet", "provider=binance", "base=BTC", "quote=USDT", "year=2024", "candles.parquet")
	if p != want {
		t.Fatalf("got %q want %q", p, want)
	}
}

func TestYearsSpanning(t *testing.T) {
	from := time.Date(2023, 12, 31, 23, 0, 0, 0, time.UTC).UnixMilli()
	to := time.Date(2024, 1, 1, 2, 0, 0, 0, time.UTC).UnixMilli()
	got := YearsSpanning(from, to)
	if len(got) != 2 || got[0] != 2023 || got[1] != 2024 {
		t.Fatalf("got %v", got)
	}
}

func TestListCandleParquetPaths(t *testing.T) {
	dir := t.TempDir()
	p2023, err := ParquetPath(dir, "binance", "BTC", "USDT", 2023)
	if err != nil {
		t.Fatal(err)
	}
	p2024, err := ParquetPath(dir, "binance", "BTC", "USDT", 2024)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{p2024, p2023} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ListCandleParquetPaths(dir, "binance", "BTC", "USDT")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != p2023 || got[1] != p2024 {
		t.Fatalf("got %v", got)
	}
}

func TestNormalizeMarketRejectsSame(t *testing.T) {
	if _, err := NormalizeMarket("BTC", "BTC"); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidatePartitionValue(t *testing.T) {
	if err := ValidatePartitionValue("binance"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "a/b", "a=b"} {
		if err := ValidatePartitionValue(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}
