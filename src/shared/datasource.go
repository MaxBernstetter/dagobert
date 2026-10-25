package shared

import (
	"encoding/json"
	"fmt"
)

// DataSource identifies where raw candles in the lake come from.
type DataSource uint8

const (
	DataSourceBinance DataSource = 1
)

// String returns the persisted provider id (e.g. kpi_revision.provider).
func (d DataSource) String() string {
	switch d {
	case DataSourceBinance:
		return "binance"
	default:
		return ""
	}
}

// ParseDataSource maps a stored provider id to a DataSource.
func ParseDataSource(s string) (DataSource, bool) {
	switch s {
	case "binance":
		return DataSourceBinance, true
	default:
		return 0, false
	}
}

// MarshalJSON writes the provider id string.
func (d DataSource) MarshalJSON() ([]byte, error) {
	if d.String() == "" {
		return []byte("null"), nil
	}
	return json.Marshal(d.String())
}

// UnmarshalJSON reads a provider id string.
func (d *DataSource) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if s == "" {
		*d = 0
		return nil
	}
	parsed, ok := ParseDataSource(s)
	if !ok {
		return fmt.Errorf("unknown data source: %q", s)
	}
	*d = parsed
	return nil
}

// DefaultDataSource is used when lake footer metadata has no data_source (legacy files).
func DefaultDataSource() DataSource {
	return DataSourceBinance
}
