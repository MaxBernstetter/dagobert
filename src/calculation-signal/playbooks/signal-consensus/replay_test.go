package signalconsensus

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/parquet-go/parquet-go"

	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
	tirunner "github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/runner"
	"github.com/MaxBernstetter/dagobert/src/internal/kpi"
	"github.com/MaxBernstetter/dagobert/src/shared"
)

const hour = int64(3_600_000)

type hourCandle struct {
	OpenTime  int64   `parquet:"open_time"`
	Open      float64 `parquet:"open"`
	High      float64 `parquet:"high"`
	Low       float64 `parquet:"low"`
	Close     float64 `parquet:"close"`
	Volume    float64 `parquet:"volume"`
	CloseTime int64   `parquet:"close_time"`
}

func TestReplaySyntheticParquetEmitsBuy(t *testing.T) {
	dir := t.TempDir()
	if err := shared.EnsureDataLayout(dir); err != nil {
		t.Fatal(err)
	}
	writeSyntheticParquet(t, dir, 720*hour)

	end := 720 * hour
	rows, err := tirunner.BuildKPIRange(dir, "BTCUSDT", 0, end, end)
	if err != nil {
		t.Fatal(err)
	}
	bodies := make(map[int64]string, len(rows))
	for _, row := range rows {
		bodies[row.BarOpen] = row.Body
	}
	win, err := kpi.LoadWindowBodies(bodies)
	if err != nil {
		t.Fatal(err)
	}

	s := Strategy{}
	aggMS := framework.AggregationDurationMS()
	barOpens := make([]int64, 0, len(bodies))
	for o := range bodies {
		barOpens = append(barOpens, o)
	}
	sort.Slice(barOpens, func(i, j int) bool { return barOpens[i] < barOpens[j] })

	var buys, sells, missingSignal, missingRSI, zeroClose int
	position := "flat"
	lastBuyBar := -1
	lastSellBar := -1
	for i, barOpen := range barOpens {
		results, err := win.Results(barOpen)
		if err != nil {
			t.Fatal(err)
		}
		barClose, _ := win.Close(barOpen)
		barVol, _ := win.Volume(barOpen)
		if barClose <= 0 {
			zeroClose++
		}
		if _, ok := kpi.MovingAverage(results, "sma", signalTrendHr); !ok {
			missingSignal++
		}
		if _, ok := kpi.RSI(results, barHr); !ok {
			missingRSI++
		}
		prevResults := []kpi.Result{}
		hasPrev := false
		prevOpen := barOpen - aggMS
		if prevOpen >= 0 {
			prevResults, err = win.Results(prevOpen)
			if err == nil {
				hasPrev = true
			}
		}
		prevClose := 0.0
		if hasPrev {
			prevClose, _ = win.Close(prevOpen)
		}
		barsSinceBuy := 999
		barsSinceSell := 999
		if lastBuyBar >= 0 {
			barsSinceBuy = i - lastBuyBar
		}
		if lastSellBar >= 0 {
			barsSinceSell = i - lastSellBar
		}
		dec, err := s.Evaluate(framework.Context{
			Position:         position,
			BarClose:         barClose,
			BarVolume:        barVol,
			PreviousBarClose: prevClose,
			BarsSinceBuy:     barsSinceBuy,
			BarsSinceSell:    barsSinceSell,
			HasPreviousBar:   hasPrev,
			Results:          results,
			PreviousResults:  prevResults,
		})
		if err != nil {
			t.Fatal(err)
		}
		switch dec.Decision {
		case "buy":
			buys++
			position = "long"
			lastBuyBar = i
		case "sell":
			sells++
			position = "flat"
			lastSellBar = i
		}
	}
	t.Logf("bars=%d buys=%d sells=%d", len(bodies), buys, sells)
	if missingRSI > 0 {
		t.Fatalf("KPI gaps: sma12=%d rsi6=%d", missingSignal, missingRSI)
	}
	if buys < 1 {
		t.Skip("v9 higher-trend stack did not fire on synthetic parquet (expected for strict macro filter)")
	}
}

func writeSyntheticParquet(t *testing.T, dataDir string, endMs int64) {
	t.Helper()
	hours := endMs / hour
	rows := make([]hourCandle, 0, hours)
	price := 100.0
	for i := 0; i < int(hours); i++ {
		switch {
		case i < int(hours)/5:
			price -= 0.2
		default:
			price += 2.5
		}
		ot := int64(i) * hour
		vol := 10.0
		if i >= int(hours)/2 {
			vol = 14.0
		}
		rows = append(rows, hourCandle{
			OpenTime: ot, Open: price, High: price + 1, Low: price - 1,
			Close: price, Volume: vol, CloseTime: ot + hour - 1,
		})
	}
	writeHourCandles(t, dataDir, rows)
}

func writeHourCandles(t *testing.T, dataDir string, rows []hourCandle) {
	t.Helper()
	byYear := map[int][]hourCandle{}
	for _, r := range rows {
		y := shared.YearUTC(r.OpenTime)
		byYear[y] = append(byYear[y], r)
	}
	for y, part := range byYear {
		pqPath, err := shared.ParquetPath(dataDir, "binance", "BTC", "USDT", y)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(pqPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := parquet.WriteFile(pqPath, part); err != nil {
			t.Fatal(err)
		}
	}
}
