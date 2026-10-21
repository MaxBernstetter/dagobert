package job

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"

	"github.com/MaxBernstetter/trading-copilot/services/process/internal/bars"
	"github.com/MaxBernstetter/trading-copilot/services/process/internal/indicators"
	"github.com/MaxBernstetter/trading-copilot/services/process/internal/lake"
	"github.com/MaxBernstetter/trading-copilot/services/process/internal/store"
)

const (
	detailWritten = 0
	detailExists  = 1
	detailInvalid = 10
	detailOpen    = 20
	detailMissing = 21
	detailFailed  = 30
	detailSchema  = 40
)

var symbols = map[string]struct{}{
	"BTCUSDT":  {},
	"ETHUSDT":  {},
	"USDCUSDT": {},
}

type kpiBody struct {
	Open      float64             `json:"open"`
	High      float64             `json:"high"`
	Low       float64             `json:"low"`
	Close     float64             `json:"close"`
	Volume    float64             `json:"volume"`
	OpenTime  int64               `json:"open_time"`
	CloseTime int64               `json:"close_time"`
	Results   []indicators.Result `json:"results"`
}

// Run executes one process invocation. detail is the M2 code. code is the
// process status: 0 succeeded, 1 failed, 2 aborted.
func Run(args []string, dataDir string, nowMS int64, nowInvalid bool) (detail int, code int) {
	if len(args) == 3 {
		return runRange(args, dataDir, nowMS, nowInvalid)
	}
	symbol, barOpen, ok := parseArgs(args, dataDir, nowInvalid)
	if !ok {
		return detailInvalid, exitCode(detailInvalid)
	}

	db, err := store.Open(filepath.Join(dataDir, "process.sqlite"))
	if err != nil {
		return detailFailed, exitCode(detailFailed)
	}
	defer db.Close()

	exists, err := db.Has(symbol, barOpen)
	if err != nil {
		return detailFailed, exitCode(detailFailed)
	}
	if exists {
		return detailExists, exitCode(detailExists)
	}

	if nowMS < barOpen+bars.BarMS {
		return detailOpen, exitCode(detailOpen)
	}

	candles, err := lake.Read(filepath.Join(dataDir, "parquet", symbol+".parquet"))
	if err != nil {
		if errors.Is(err, lake.ErrMissing) {
			return detailMissing, exitCode(detailMissing)
		}
		if errors.Is(err, lake.ErrSchema) {
			return detailSchema, exitCode(detailSchema)
		}
		return detailFailed, exitCode(detailFailed)
	}

	series, ok := bars.Series(candles, barOpen)
	if !ok {
		return detailMissing, exitCode(detailMissing)
	}

	results, err := indicators.Results(series)
	if err != nil {
		return detailFailed, exitCode(detailFailed)
	}

	current := series[len(series)-1]
	encoded, err := json.Marshal(kpiBody{
		Open:      current.Open,
		High:      current.High,
		Low:       current.Low,
		Close:     current.Close,
		Volume:    current.Volume,
		OpenTime:  current.OpenTime,
		CloseTime: current.CloseTime,
		Results:   results,
	})
	if err != nil {
		return detailFailed, exitCode(detailFailed)
	}

	if err := db.Insert(symbol, barOpen, string(encoded), barSource("live")); err != nil {
		if errors.Is(err, store.ErrExists) {
			return detailExists, exitCode(detailExists)
		}
		return detailFailed, exitCode(detailFailed)
	}
	return detailWritten, exitCode(detailWritten)
}

func runRange(args []string, dataDir string, nowMS int64, nowInvalid bool) (detail int, code int) {
	if nowInvalid || dataDir == "" {
		return detailInvalid, exitCode(detailInvalid)
	}
	symbol := args[0]
	if _, ok := symbols[symbol]; !ok {
		return detailInvalid, exitCode(detailInvalid)
	}
	from, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return detailInvalid, exitCode(detailInvalid)
	}
	to, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil || from >= to {
		return detailInvalid, exitCode(detailInvalid)
	}
	if rem := from % bars.BarMS; rem != 0 {
		from += bars.BarMS - rem
	}

	db, err := store.Open(filepath.Join(dataDir, "process.sqlite"))
	if err != nil {
		return detailFailed, exitCode(detailFailed)
	}
	defer db.Close()

	candles, err := lake.Read(filepath.Join(dataDir, "parquet", symbol+".parquet"))
	if err != nil {
		if errors.Is(err, lake.ErrMissing) {
			return detailMissing, exitCode(detailMissing)
		}
		if errors.Is(err, lake.ErrSchema) {
			return detailSchema, exitCode(detailSchema)
		}
		return detailFailed, exitCode(detailFailed)
	}

	ordered := bars.Ordered(candles)
	window := indicators.Window()
	rows := make([]store.Row, 0, len(ordered))
	for i, bar := range ordered {
		if bar.OpenTime < from || bar.OpenTime+bars.BarMS > to || bar.OpenTime+bars.BarMS > nowMS {
			continue
		}
		start := i - window + 1
		if start < 0 {
			start = 0
		}
		results, err := indicators.Results(ordered[start : i+1])
		if err != nil {
			return detailFailed, exitCode(detailFailed)
		}
		encoded, err := json.Marshal(kpiBody{
			Open:      bar.Open,
			High:      bar.High,
			Low:       bar.Low,
			Close:     bar.Close,
			Volume:    bar.Volume,
			OpenTime:  bar.OpenTime,
			CloseTime: bar.CloseTime,
			Results:   results,
		})
		if err != nil {
			return detailFailed, exitCode(detailFailed)
		}
		rows = append(rows, store.Row{Symbol: symbol, BarOpen: bar.OpenTime, Body: string(encoded), Source: barSource("backfill")})
	}
	if len(rows) == 0 {
		return detailMissing, exitCode(detailMissing)
	}
	if err := db.UpsertAll(rows); err != nil {
		return detailFailed, exitCode(detailFailed)
	}
	return detailWritten, exitCode(detailWritten)
}

func barSource(fallback string) string {
	switch os.Getenv("SOURCE") {
	case "live", "backfill":
		return os.Getenv("SOURCE")
	default:
		return fallback
	}
}

func parseArgs(args []string, dataDir string, nowInvalid bool) (string, int64, bool) {
	if nowInvalid || dataDir == "" || len(args) != 2 {
		return "", 0, false
	}
	symbol := args[0]
	if _, ok := symbols[symbol]; !ok {
		return "", 0, false
	}
	barOpen, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil || barOpen < 0 || barOpen%bars.BarMS != 0 {
		return "", 0, false
	}
	return symbol, barOpen, true
}

func exitCode(detail int) int {
	switch detail {
	case detailWritten:
		return 0
	case detailExists, detailOpen, detailMissing:
		return 2
	default:
		return 1
	}
}
