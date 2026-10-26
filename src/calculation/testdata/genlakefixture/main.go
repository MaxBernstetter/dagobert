// Writes tests/fixtures/lake Hive parquet for dagobert-tests.
package main

import (
	"flag"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"

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
	benchDir := flag.String("bench", "", "write only a seeded benchmark lake (provider=binance, BTC/USDT) under this directory")
	benchBars := flag.Int("bars", 5840, "bench: number of 3h bars (5840 = 730 days)")
	benchSeed := flag.Int64("seed", 110, "bench: random seed")
	flag.Parse()
	if *benchDir != "" {
		writeBenchLake(*benchDir, *benchBars, *benchSeed)
		return
	}
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
	writeLakeInterval("lake_consensus", consensusRows(openMs), 3600000)
}

// consensusRows is the 56-bar convex uptrend with dips as 168 hourly candles, the
// source interval of the real lake. The strategy aggregates them to 3h bars whose
// closes are the path below; inside a bar the hourly closes move evenly from the
// previous bar close, so rsi-3h reads 100 on rising bars and 0 on falling ones.
// It trips consensus_buy, consensus_sell and atr_stop (see consensus_exe_test.cpp).
func consensusRows(openMs int64) []candle {
	const hourMs int64 = 3600000
	dips := map[int]float64{
		12: 4, 18: 4, 24: 4, 30: 4, 36: 4, 42: 4,
		46: 6, 48: 14, 49: 20, 50: 20, 51: 60, 52: 60, 53: 60, 54: 60, 55: 60,
	}
	barClose := func(i int) float64 {
		return 100 + float64(i) + 0.015*float64(i*i) - dips[i]
	}
	var rows []candle
	prev := barClose(0)
	for i := 0; i < 56; i++ {
		end := barClose(i)
		for k := 0; k < 3; k++ {
			c := prev + (end-prev)*float64(k+1)/3
			open := openMs + (int64(i)*3+int64(k))*hourMs
			rows = append(rows, candle{
				open, fmtF(prev), fmtF(math.Max(prev, c) + 0.5), fmtF(math.Min(prev, c) - 0.5), fmtF(c), "1",
				open + hourMs - 1,
			})
			prev = c
		}
	}
	return rows
}

func fmtF(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

func writeLake(name string, rows []candle) { writeLakeInterval(name, rows, 10800000) }

func writeLakeInterval(name string, rows []candle, intervalMs int64) {
	out := filepath.Join("src", "calculation", "tests", "fixtures", name,
		"parquet", "provider=demo", "base=BTC", "quote=USDT", "year=2024",
		"candles.parquet")
	writeFileInterval(out, rows, intervalMs)
}

func writeFile(out string, rows []candle) { writeFileInterval(out, rows, 10800000) }

func writeFileInterval(out string, rows []candle, intervalMs int64) {
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		panic(err)
	}
	if err := parquet.WriteFile(out, rows,
		parquet.KeyValueMetadata("trading_copilot.schema_version", "1"),
		parquet.KeyValueMetadata("trading_copilot.interval_ms", strconv.FormatInt(intervalMs, 10)),
		parquet.KeyValueMetadata("trading_copilot.data_source", "binance"),
	); err != nil {
		panic(err)
	}
}

// writeBenchLake writes a deterministic geometric random walk of 3h bars
// starting 2024-01-01 UTC into <dir>/parquet, one file per UTC year.
func writeBenchLake(dir string, bars int, seed int64) {
	const intervalMs int64 = 10800000
	openMs := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	rng := rand.New(rand.NewSource(seed))
	price := 40000.0
	byYear := map[int][]candle{}
	for i := 0; i < bars; i++ {
		open := openMs + int64(i)*intervalMs
		o := price
		price *= math.Exp(0.0003 + 0.012*rng.NormFloat64())
		hi := math.Max(o, price) * (1 + 0.004*rng.Float64())
		lo := math.Min(o, price) * (1 - 0.004*rng.Float64())
		year := time.UnixMilli(open).UTC().Year()
		byYear[year] = append(byYear[year], candle{
			open, fmtF(o), fmtF(hi), fmtF(lo), fmtF(price),
			fmtF(100 + 50*rng.Float64()), open + intervalMs - 1,
		})
	}
	for year, rows := range byYear {
		out := filepath.Join(dir, "parquet", "provider=binance", "base=BTC",
			"quote=USDT", "year="+strconv.Itoa(year), "candles.parquet")
		writeFile(out, rows)
	}
}
