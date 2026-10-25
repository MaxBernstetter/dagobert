// Writes tests/fixtures/lake Hive parquet for dagobert-tests.
package main

import (
	"os"
	"path/filepath"
	"strconv"

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
	const openMs int64 = 1704067200000
	const intervalMs int64 = 10800000
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
		open := openMs + int64(i)*intervalMs
		rows = append(rows, candle{open, c, c, c, c, "1", open + intervalMs - 1})
	}
	writeLake("lake_roundtrip", rows)
	writeLake("lake_consensus", consensusRows(openMs, intervalMs))
}

// consensusRows is a 56-bar convex uptrend with dips. It trips consensus_buy at
// bar 19 (MACD histogram not ready yet), consensus_sell at 24, a second buy at
// 47 (first recovery after the eight-bar cooldown with a positive histogram)
// and atr_stop at 51, where the price is already below its 12h mean.
func consensusRows(openMs, intervalMs int64) []candle {
	dips := map[int]float64{
		12: 4, 18: 4, 24: 4, 30: 4, 36: 4, 42: 4,
		46: 6, 48: 14, 49: 20, 50: 20, 51: 60, 52: 60, 53: 60, 54: 60, 55: 60,
	}
	var rows []candle
	for i := 0; i < 56; i++ {
		c := 100 + float64(i) + 0.015*float64(i*i) - dips[i]
		open := openMs + int64(i)*intervalMs
		rows = append(rows, candle{
			open, fmtF(c), fmtF(c + 0.5), fmtF(c - 0.5), fmtF(c), "1",
			open + intervalMs - 1,
		})
	}
	return rows
}

func fmtF(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

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
