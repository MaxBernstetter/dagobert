package indicators

import (
	"encoding/json"

	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/bars"
)

// Units say what a result is compared with. A playbook may compare two results
// only when the unit matches; nothing rescales a price into an index.
//
//	price   same unit as the bar close
//	amount  a quantity, such as base-asset volume
//	ratio   a change relative to a base, not a price
//	index   a bounded scale, such as RSI
//
// A definition is a parameter object (type plus parameters, `time_frame_hr` a
// positive multiple of 3), not a column name. It is stored canonically (keys
// sorted, no extra keys, integer numbers); objects with the same canonical text
// are one node in the calculation graph.
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
