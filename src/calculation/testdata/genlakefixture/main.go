// Writes tests/fixtures/lake Hive parquet for dagobert-tests.
package main

import (
	"os"
	"path/filepath"

	"github.com/parquet-go/parquet-go"
)

type candle struct {
	OpenTime  int64  `parquet:"open_time"`
	Open      string `parquet:"open"`
	High      string `parquet:"high"`
	Low       string `parquet:"low"`
	Close     string `parquet:"close"`
	Volume    string `parquet:"volume"`
	CloseTime int64  `parquet:"close_time"`
}

func main() {
	const openMs = 1704067200000
	const intervalMs = 10800000
	row := candle{
		OpenTime:  openMs,
		Open:      "100.25",
		High:      "101.50",
		Low:       "99.75",
		Close:     "100.50",
		Volume:    "12.345",
		CloseTime: openMs + intervalMs - 1,
	}
	writeLake("lake", []candle{row})

	// Twelve bars that open and close one sma-cross trade (fast 3h, slow 6h).
	closes := []string{"10", "10", "10", "10", "20", "20", "20", "20", "10", "10", "10", "10"}
	var rows []candle
	for i, c := range closes {
		open := int64(openMs + i*intervalMs)
		rows = append(rows, candle{open, c, c, c, c, "1", open + intervalMs - 1})
	}
	writeLake("lake_roundtrip", rows)
}

func writeLake(name string, rows []candle) {
	out := filepath.Join("src", "calculation", "tests", "fixtures", name,
		"parquet", "provider=demo", "base=BTC", "quote=USDT", "year=2024",
		"candles.parquet")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		panic(err)
	}
	if err := parquet.WriteFile(out, rows,
		parquet.KeyValueMetadata("trading_copilot.schema_version", "1"),
		parquet.KeyValueMetadata("trading_copilot.interval_ms", "10800000"),
		parquet.KeyValueMetadata("trading_copilot.data_source", "binance"),
	); err != nil {
		panic(err)
	}
}
