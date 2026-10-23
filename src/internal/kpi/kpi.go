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

var ErrNotFound = errors.New("kpi row not found")

// Result is one indicator value from a KPI row body.
type Result struct {
	Def   json.RawMessage
	Value json.RawMessage
	Unit  string
}

// Read loads the KPI body for symbol and bar open from technical_indicator.sqlite.
func Read(dataDir, symbol string, barOpen int64) ([]Result, error) {
	path := filepath.Join(dataDir, "technical_indicator.sqlite")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("technical_indicator.sqlite missing: %w", err)
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
		Results []Result `json:"results"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return nil, fmt.Errorf("parse kpi body: %w", err)
	}
	return parsed.Results, nil
}

// ReadSnapshot loads close, volume, and indicator results for one bar.
func ReadSnapshot(dataDir, symbol string, barOpen int64) (BarSnapshot, error) {
	path := filepath.Join(dataDir, "technical_indicator.sqlite")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return BarSnapshot{}, fmt.Errorf("technical_indicator.sqlite missing: %w", err)
		}
		return BarSnapshot{}, err
	}

	dsn := fmt.Sprintf("file:%s?mode=ro", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return BarSnapshot{}, err
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
		return BarSnapshot{}, ErrNotFound
	}
	if err != nil {
		return BarSnapshot{}, err
	}

	var parsed struct {
		Close   float64  `json:"close"`
		Volume  float64  `json:"volume"`
		Results []Result `json:"results"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return BarSnapshot{}, fmt.Errorf("parse kpi body: %w", err)
	}
	return BarSnapshot{Close: parsed.Close, Volume: parsed.Volume, Results: parsed.Results}, nil
}
