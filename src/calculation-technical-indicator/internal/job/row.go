package job

import (
	"fmt"

	"github.com/MaxBernstetter/dagobert/src/shared"
)

// Algo is the indicator algorithm version of the computed rows.
const Algo = "v1"

// AlgoSpec records the parameters of Algo for the given candle data source.
func AlgoSpec(source shared.DataSource) string {
	return fmt.Sprintf(
		`{"id":"v1","price_source":"%s","sma_hours":[3,6],"rsi_hours":3}`,
		source.String(),
	)
}

// Row is one computed indicator bar: the KPI body (bar OHLCV plus indicator results) as JSON.
type Row struct {
	Symbol     string
	BarOpen    int64
	Body       string
	Source     string
	DataSource shared.DataSource
}
