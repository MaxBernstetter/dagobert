package kpi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// BarClose returns the candle close from the KPI row at barOpen.
func BarClose(dataDir, symbol string, barOpen int64) (float64, error) {
	body, err := readBody(dataDir, symbol, barOpen)
	if err != nil {
		return 0, err
	}
	var parsed struct {
		Close float64 `json:"close"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, fmt.Errorf("parse kpi close: %w", err)
	}
	if parsed.Close <= 0 {
		return 0, errors.New("invalid close")
	}
	return parsed.Close, nil
}

func readBody(dataDir, symbol string, barOpen int64) ([]byte, error) {
	path := filepath.Join(dataDir, "technical_indicator.sqlite")
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var body string
	err = db.QueryRow(
		`SELECT body FROM kpi_revision WHERE symbol = ? AND bar_open = ? AND algo = 'v1'`,
		symbol, barOpen,
	).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
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
	return []byte(body), nil
}
