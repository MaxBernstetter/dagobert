package job

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/bars"
	indicatorrun "github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/indicators/run"
	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/lake"
	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/store"
	"github.com/MaxBernstetter/dagobert/src/shared"
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

// validPipelineSymbol accepts any lake series that splits into provider base/quote
// (no hardcoded BTCUSDT allowlist — catalog membership is enforced at pick/ingest).
func validPipelineSymbol(symbol string) bool {
	_, _, ok := shared.SplitPipelineSymbol(strings.TrimSpace(symbol))
	return ok
}

type kpiBody struct {
	Open      float64             `json:"open"`
	High      float64             `json:"high"`
	Low       float64             `json:"low"`
	Close     float64             `json:"close"`
	Volume    float64             `json:"volume"`
	OpenTime  int64               `json:"open_time"`
	CloseTime int64               `json:"close_time"`
	Results   []indicatorrun.Result `json:"results"`
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

	db, err := store.Open(filepath.Join(dataDir, "technical_indicator.sqlite"))
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

	raw, err := lake.Read(rawParquetPaths(dataDir, symbol)...)
	if err != nil {
		if errors.Is(err, lake.ErrMissing) {
			return detailMissing, exitCode(detailMissing)
		}
		if errors.Is(err, lake.ErrSchema) {
			return detailSchema, exitCode(detailSchema)
		}
		return detailFailed, exitCode(detailFailed)
	}

	spec := AggregationSpec(raw.IntervalMS)
	if barOpen%spec.DurationMS != 0 {
		return detailInvalid, exitCode(detailInvalid)
	}

	if nowMS < barOpen+spec.DurationMS {
		return detailOpen, exitCode(detailOpen)
	}

	series, ok := bars.Series(spec, raw.Candles, barOpen)
	if !ok {
		return detailMissing, exitCode(detailMissing)
	}

	results, err := indicatorrun.Results(series)
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

	if err := db.Insert(symbol, barOpen, string(encoded), barSource("live"), raw.DataSource); err != nil {
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
	if !validPipelineSymbol(symbol) {
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

	rows, err := BuildRangeRows(dataDir, symbol, from, to, nowMS)
	if err != nil {
		if errors.Is(err, lake.ErrMissing) {
			return detailMissing, exitCode(detailMissing)
		}
		if errors.Is(err, lake.ErrSchema) {
			return detailSchema, exitCode(detailSchema)
		}
		return detailFailed, exitCode(detailFailed)
	}
	for i := range rows {
		rows[i].Source = barSource("backfill")
	}

	db, err := store.Open(filepath.Join(dataDir, "technical_indicator.sqlite"))
	if err != nil {
		return detailFailed, exitCode(detailFailed)
	}
	defer db.Close()
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
	if !validPipelineSymbol(symbol) {
		return "", 0, false
	}
	barOpen, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil || barOpen < 0 {
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
