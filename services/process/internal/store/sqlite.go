package store

import (
	"database/sql"
	"errors"
	"strings"

	_ "modernc.org/sqlite"
)

// ErrExists means the KPI primary key is already stored.
var ErrExists = errors.New("kpi exists")

const createTable = `
CREATE TABLE IF NOT EXISTS kpi (
  symbol TEXT NOT NULL,
  bar_open INTEGER NOT NULL,
  body TEXT NOT NULL,
  PRIMARY KEY (symbol, bar_open)
)`

// Store is the process KPI database.
type Store struct {
	db *sql.DB
}

// Open opens DATA_DIR/process.sqlite and ensures the kpi table. It does not
// read parquet. Open is retried three times.
func Open(path string) (*Store, error) {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		db, err := sql.Open("sqlite", path)
		if err != nil {
			last = err
			continue
		}
		if _, err = db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
			db.Close()
			last = err
			continue
		}
		if _, err = db.Exec(createTable); err != nil {
			db.Close()
			last = err
			continue
		}
		return &Store{db: db}, nil
	}
	return nil, last
}

// Close releases the database handle.
func (s *Store) Close() error {
	return s.db.Close()
}

// Has reports whether the symbol and bar open already have a row.
func (s *Store) Has(symbol string, barOpen int64) (bool, error) {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		var one int
		err := s.db.QueryRow(
			`SELECT 1 FROM kpi WHERE symbol = ? AND bar_open = ?`,
			symbol, barOpen,
		).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			last = err
			continue
		}
		return true, nil
	}
	return false, last
}

// Insert writes the row when the key is absent. A duplicate key is ErrExists.
// The statement is retried three times.
func (s *Store) Insert(symbol string, barOpen int64, body string) error {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		_, err := s.db.Exec(
			`INSERT INTO kpi (symbol, bar_open, body) VALUES (?, ?, ?)`,
			symbol, barOpen, body,
		)
		if err == nil {
			return nil
		}
		if strings.Contains(err.Error(), "UNIQUE") {
			return ErrExists
		}
		last = err
	}
	return last
}
