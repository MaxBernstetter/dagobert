package lakeobject

import (
	"path/filepath"
	"testing"
)

func TestObjectKey(t *testing.T) {
	dataDir := "/data"
	abs := filepath.Join(dataDir, "parquet", "provider=binance", "base=BTC", "quote=USDT", "year=2024", "candles.parquet")
	key, err := ObjectKey(dataDir, abs)
	if err != nil {
		t.Fatal(err)
	}
	if key != "parquet/provider=binance/base=BTC/quote=USDT/year=2024/candles.parquet" {
		t.Fatalf("key: %q", key)
	}
	_, err = ObjectKey(dataDir, filepath.Join(dataDir, "experiments", "x"))
	if err == nil {
		t.Fatal("expected error for non-parquet path")
	}
}
