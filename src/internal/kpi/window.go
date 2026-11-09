package kpi

import (
	"encoding/json"
	"fmt"
)

// BarSnapshot is one aggregated bar from computed KPI rows.
type BarSnapshot struct {
	Close   float64
	Volume  float64
	Results []Result
	// ByKey maps canonical indicator def keys → float (built at load).
	ByKey map[string]float64
}

// Window holds KPI bodies keyed by bar_open for in-memory replay.
type Window struct {
	byOpen map[int64]BarSnapshot
}

// LoadWindowBodies builds a Window from preloaded KPI JSON bodies keyed by bar_open.
func LoadWindowBodies(bodies map[int64]string) (*Window, error) {
	w := &Window{byOpen: make(map[int64]BarSnapshot)}
	for barOpen, body := range bodies {
		var parsed struct {
			Close   float64  `json:"close"`
			Volume  float64  `json:"volume"`
			Results []Result `json:"results"`
		}
		if err := json.Unmarshal([]byte(body), &parsed); err != nil {
			return nil, fmt.Errorf("parse kpi bar_open=%d: %w", barOpen, err)
		}
		w.byOpen[barOpen] = BarSnapshot{
			Close:   parsed.Close,
			Volume:  parsed.Volume,
			Results: parsed.Results,
			ByKey:   IndexResults(parsed.Results),
		}
	}
	if len(w.byOpen) == 0 {
		return nil, ErrNotFound
	}
	return w, nil
}

// Snapshot returns the bar snapshot when present.
func (w *Window) Snapshot(barOpen int64) (BarSnapshot, bool) {
	if w == nil {
		return BarSnapshot{}, false
	}
	s, ok := w.byOpen[barOpen]
	return s, ok
}

func (w *Window) Results(barOpen int64) ([]Result, error) {
	if w == nil {
		return nil, ErrNotFound
	}
	s, ok := w.byOpen[barOpen]
	if !ok {
		return nil, ErrNotFound
	}
	return s.Results, nil
}

func (w *Window) Close(barOpen int64) (float64, error) {
	if w == nil {
		return 0, ErrNotFound
	}
	s, ok := w.byOpen[barOpen]
	if !ok || s.Close <= 0 {
		return 0, ErrNotFound
	}
	return s.Close, nil
}

func (w *Window) Volume(barOpen int64) (float64, error) {
	if w == nil {
		return 0, ErrNotFound
	}
	s, ok := w.byOpen[barOpen]
	if !ok || s.Volume <= 0 {
		return 0, ErrNotFound
	}
	return s.Volume, nil
}
