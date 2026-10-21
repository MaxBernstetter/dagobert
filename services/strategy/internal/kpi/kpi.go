package kpi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/MaxBernstetter/trading-copilot/services/strategy/internal/playbook"
)

var ErrNotFound = errors.New("kpi row not found")

// Read loads the KPI body for symbol and bar open from process.sqlite.
func Read(dataDir, symbol string, barOpen int64) ([]playbook.KPIResult, error) {
	path := filepath.Join(dataDir, "process.sqlite")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("process.sqlite missing: %w", err)
		}
		return nil, err
	}

	dsn := fmt.Sprintf("file:%s?mode=ro", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var body string
	err = db.QueryRow(
		`SELECT body FROM kpi_revision WHERE symbol = ? AND bar_open = ? AND algo = 'v1'`,
		symbol, barOpen,
	).Scan(&body)
	if err != nil {
		err = db.QueryRow(
			`SELECT body FROM kpi WHERE symbol = ? AND bar_open = ?`,
			symbol, barOpen,
		).Scan(&body)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Results []struct {
			Def   json.RawMessage `json:"def"`
			Value json.RawMessage `json:"value"`
			Unit  string          `json:"unit"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return nil, fmt.Errorf("parse kpi body: %w", err)
	}

	out := make([]playbook.KPIResult, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		out = append(out, playbook.KPIResult{
			Def:   r.Def,
			Value: r.Value,
			Unit:  r.Unit,
		})
	}
	return out, nil
}
