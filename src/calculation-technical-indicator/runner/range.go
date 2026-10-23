package runner

import (
	"errors"

	"github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/job"
	"github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/lake"
	"github.com/MaxBernstetter/trading-copilot/src/shared"
)

// KPIRow is a computed indicator bar for backtest storage.
type KPIRow struct {
	Symbol     string
	BarOpen    int64
	Body       string
	Source     string
	DataSource shared.DataSource
}

// BuildKPIRange reads parquet and returns KPI rows for [from, to) without writing prod TI.
func BuildKPIRange(dataDir, symbol string, from, to, nowMS int64) ([]KPIRow, error) {
	rows, err := job.BuildRangeRows(dataDir, symbol, from, to, nowMS)
	if err != nil {
		return nil, err
	}
	out := make([]KPIRow, len(rows))
	for i, row := range rows {
		out[i] = KPIRow{
			Symbol:     row.Symbol,
			BarOpen:    row.BarOpen,
			Body:       row.Body,
			Source:     row.Source,
			DataSource: row.DataSource,
		}
	}
	return out, nil
}

// IsMissingParquet reports whether the error is missing candle data.
func IsMissingParquet(err error) bool {
	return errors.Is(err, lake.ErrMissing)
}
