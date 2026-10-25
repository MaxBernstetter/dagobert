package kpi

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// BarSnapshot is one aggregated bar from technical_indicator storage.
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

// LoadWindow reads kpi_revision rows for symbol and bar_open in [minOpen, maxOpen].
func LoadWindow(dataDir, symbol string, minOpen, maxOpen int64) (*Window, error) {
	path := filepath.Join(dataDir, "technical_indicator.sqlite")
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(
		`SELECT bar_open, body FROM kpi_revision WHERE symbol = ? AND algo = 'v1' AND bar_open >= ? AND bar_open <= ? ORDER BY bar_open`,
		symbol, minOpen, maxOpen,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	w := &Window{byOpen: make(map[int64]BarSnapshot)}
	for rows.Next() {
		var barOpen int64
		var body string
		if err := rows.Scan(&barOpen, &body); err != nil {
			return nil, err
		}
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
	return w, rows.Err()
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
