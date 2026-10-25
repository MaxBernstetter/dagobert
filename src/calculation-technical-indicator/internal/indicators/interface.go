package indicators

import (
	"encoding/json"

	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/bars"
)

const (
	UnitPrice  = "price"
	UnitAmount = "amount"
	UnitRatio  = "ratio"
	UnitIndex  = "index"
)

// TechnicalIndicator is one indicator formula. timeFrameHr is the lookback window in hours.
type TechnicalIndicator interface {
	Slug() string
	Unit() string
	Definition(timeFrameHr int) any
	Calculate(series []bars.CandleAggregation, timeFrameHr int) (float64, bool)
}

// Result is one graph node stored on the KPI row.
type Result struct {
	Def   json.RawMessage `json:"def"`
	Value float64         `json:"value"`
	Unit  string          `json:"unit"`
}

// Deployment binds a series key to a formula and a time_frame_hr for algorithm v1.
type Deployment struct {
	Key         string
	TimeFrameHr int
	Indicator   TechnicalIndicator
}

// MarshalDefinition JSON-encodes a KPI definition struct.
func MarshalDefinition(def any) []byte {
	raw, err := json.Marshal(def)
	if err != nil {
		return []byte("{}")
	}
	return raw
}
