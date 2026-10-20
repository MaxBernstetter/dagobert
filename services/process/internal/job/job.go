package job

import (
	"encoding/json"
	"errors"
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

	if err := db.Insert(symbol, barOpen, string(encoded)); err != nil {
		if errors.Is(err, store.ErrExists) {
			return detailExists, exitCode(detailExists)
		}
		return detailFailed, exitCode(detailFailed)
	}
	return detailWritten, exitCode(detailWritten)
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
