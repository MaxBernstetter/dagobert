package canonical

import (
	"encoding/json"
	"fmt"
)

// IndicatorDef is the normalized indicator parameter object.
type IndicatorDef struct {
	Type        string `json:"type" yaml:"type"`
	Method      string `json:"method" yaml:"method"`
	TimeFrameHr int    `json:"time_frame_hr" yaml:"time_frame_hr"`
}

// String returns canonical JSON: sorted keys, integers as integers, no extra keys.
func (d IndicatorDef) String() (string, error) {
	// Keys sorted alphabetically: method, time_frame_hr, type.
	obj := map[string]any{
		"method":         d.Method,
		"time_frame_hr":  d.TimeFrameHr,
		"type":           d.Type,
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("marshal def: %w", err)
	}
	return string(b), nil
}

// ParseDefFromJSON unmarshals a KPI or playbook def object into IndicatorDef.
func ParseDefFromJSON(raw json.RawMessage) (IndicatorDef, error) {
	var d IndicatorDef
	if err := json.Unmarshal(raw, &d); err != nil {
		return IndicatorDef{}, err
	}
	return d, nil
}
