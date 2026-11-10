package kpi

import (
	"encoding/json"
	"errors"
)

// ErrNotFound means no KPI row exists for the requested bar.
var ErrNotFound = errors.New("kpi row not found")

// Result is one indicator value from a KPI row body.
type Result struct {
	Def   json.RawMessage
	Value json.RawMessage
	Unit  string
}
