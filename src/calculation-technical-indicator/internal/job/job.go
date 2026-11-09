package job

import (
	"errors"
	"strconv"
	"strings"

	indicatorrun "github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/indicators/run"
	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/lake"
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
	Open      float64               `json:"open"`
	High      float64               `json:"high"`
	Low       float64               `json:"low"`
	Close     float64               `json:"close"`
	Volume    float64               `json:"volume"`
	OpenTime  int64                 `json:"open_time"`
	CloseTime int64                 `json:"close_time"`
	Results   []indicatorrun.Result `json:"results"`
}

// Run executes one process invocation: symbol, from, to. It computes the indicator rows for the
// closed bars in [from, to) and stores them in the TI parquet cache of the lake (the only output).
// detail is the diagnostic code. code is the process status: 0 succeeded, 1 failed, 2 aborted.
func Run(args []string, dataDir string, nowMS int64, nowInvalid bool) (detail int, code int) {
	if len(args) != 3 {
		return detailInvalid, exitCode(detailInvalid)
	}
	return runRange(args, dataDir, nowMS, nowInvalid)
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

	if _, err := buildRangeRows(dataDir, symbol, from, to, nowMS, true); err != nil {
		if errors.Is(err, lake.ErrMissing) {
			return detailMissing, exitCode(detailMissing)
		}
		if errors.Is(err, lake.ErrSchema) {
			return detailSchema, exitCode(detailSchema)
		}
		return detailFailed, exitCode(detailFailed)
	}
	return detailWritten, exitCode(detailWritten)
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
