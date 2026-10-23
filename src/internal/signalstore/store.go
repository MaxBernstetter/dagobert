package signalstore

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

var ErrDuplicateAction = errors.New("action already stored")
var ErrDuplicateSignal = errors.New("signal already stored")
var ErrNotFound = errors.New("not found")

const createActionSQL = `
CREATE TABLE IF NOT EXISTS action (
  playbook TEXT NOT NULL,
  symbol TEXT NOT NULL,
  candle_close INTEGER NOT NULL,
  body TEXT NOT NULL,
  PRIMARY KEY (playbook, symbol, candle_close)
);`

const createSignalSQL = `
CREATE TABLE IF NOT EXISTS signal (
  playbook TEXT NOT NULL,
  symbol TEXT NOT NULL,
  candle_close INTEGER NOT NULL,
  body TEXT NOT NULL,
  PRIMARY KEY (playbook, symbol, candle_close)
);`

// Open opens calculation_signal.sqlite and ensures signal/action tables exist.
func Open(dataDir string) (*sql.DB, error) {
	path := SQLitePath(dataDir)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(createActionSQL); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(createSignalSQL); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func HasAction(db *sql.DB, playbookID, symbol string, candleClose int64) (bool, error) {
	return hasRow(db, "action", playbookID, symbol, candleClose)
}

func HasSignal(db *sql.DB, playbookID, symbol string, candleClose int64) (bool, error) {
	return hasRow(db, "signal", playbookID, symbol, candleClose)
}

func hasRow(db *sql.DB, table, playbookID, symbol string, candleClose int64) (bool, error) {
	var one int
	err := db.QueryRow(
		`SELECT 1 FROM `+table+` WHERE playbook = ? AND symbol = ? AND candle_close = ?`,
		playbookID, symbol, candleClose,
	).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// Signal is a layer-1 playbook outcome (no size).
type Signal struct {
	Decision string
	RuleID   string
}

func ReadSignal(db *sql.DB, playbookID, symbol string, candleClose int64) (Signal, error) {
	var raw string
	err := db.QueryRow(
		`SELECT body FROM signal WHERE playbook = ? AND symbol = ? AND candle_close = ?`,
		playbookID, symbol, candleClose,
	).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return Signal{}, ErrNotFound
	}
	if err != nil {
		return Signal{}, err
	}
	var body struct {
		Decision string `json:"decision"`
		Rule     string `json:"rule"`
	}
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return Signal{}, err
	}
	return Signal{Decision: body.Decision, RuleID: body.Rule}, nil
}

func CommitSignal(db *sql.DB, playbookID, symbol string, candleClose int64, body []byte) error {
	_, err := db.Exec(
		`INSERT INTO signal (playbook, symbol, candle_close, body) VALUES (?, ?, ?, ?)`,
		playbookID, symbol, candleClose, string(body),
	)
	if err != nil && isUniqueViolation(err) {
		return ErrDuplicateSignal
	}
	return err
}

func CommitAction(db *sql.DB, playbookID, symbol string, candleClose int64, actionBody []byte) error {
	_, err := db.Exec(
		`INSERT INTO action (playbook, symbol, candle_close, body) VALUES (?, ?, ?, ?)`,
		playbookID, symbol, candleClose, string(actionBody),
	)
	if err != nil && isUniqueViolation(err) {
		return ErrDuplicateAction
	}
	return err
}

func isUniqueViolation(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "constraint failed")
}

// AggregationDurationMS reads the aggregation window for bar-open calculation.
func AggregationDurationMS() int64 {
	raw := strings.TrimSpace(os.Getenv("AGGREGATION_DURATION_MS"))
	if raw == "" {
		return 10_800_000
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		return 10_800_000
	}
	return v
}
