package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/MaxBernstetter/trading-copilot/src/shared"
)

// ErrExists means the KPI primary key is already stored.
var ErrExists = errors.New("kpi exists")

// Algo is the indicator algorithm version stored with each KPI revision.
const Algo = "v1"

// AlgoSpec records the parameters of Algo for the given candle data source.
func AlgoSpec(source shared.DataSource) string {
	return fmt.Sprintf(
		`{"id":"v1","price_source":"%s","sma_hours":[3,6],"rsi_hours":3}`,
		source.String(),
	)
}

const createRevision = `
CREATE TABLE IF NOT EXISTS kpi_revision (
  symbol TEXT NOT NULL,
  bar_open INTEGER NOT NULL,
  provider TEXT NOT NULL,
  algo TEXT NOT NULL,
  body TEXT NOT NULL,
  origin TEXT NOT NULL,
  PRIMARY KEY (symbol, bar_open, algo)
)`

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

// Open opens DATA_DIR/technical_indicator.sqlite and ensures the kpi table. It does not
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
		if _, err = db.Exec(`ALTER TABLE kpi ADD COLUMN source TEXT NOT NULL DEFAULT 'backfill'`); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			db.Close()
			last = err
			continue
		}
		if _, err = db.Exec(createRevision); err != nil {
			db.Close()
			last = err
			continue
		}
		if _, err = db.Exec(`ALTER TABLE kpi_revision ADD COLUMN spec TEXT NOT NULL DEFAULT ''`); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			db.Close()
			last = err
			continue
		}
		if _, err = db.Exec(`ALTER TABLE kpi ADD COLUMN origin TEXT NOT NULL DEFAULT 'backfill'`); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			db.Close()
			last = err
			continue
		}
		_, _ = db.Exec(`UPDATE kpi SET origin = source WHERE origin = 'backfill' AND source != '' AND source != origin`)
		defaultSource := shared.DefaultDataSource()
		if _, err = db.Exec(`INSERT OR IGNORE INTO kpi_revision (symbol, bar_open, provider, algo, body, origin, spec)
			SELECT symbol, bar_open, ?, 'v1', body, COALESCE(origin, source, 'backfill'), ? FROM kpi`,
			defaultSource.String(), AlgoSpec(defaultSource)); err != nil {
			db.Close()
			last = err
			continue
		}
		if _, err = db.Exec(`UPDATE kpi_revision SET spec = ? WHERE spec = ''`, AlgoSpec(defaultSource)); err != nil {
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
func (s *Store) Insert(symbol string, barOpen int64, body, origin string, dataSource shared.DataSource) error {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		_, err := s.db.Exec(
			`INSERT INTO kpi (symbol, bar_open, body, source) VALUES (?, ?, ?, ?)`,
			symbol, barOpen, body, origin,
		)
		if err == nil {
			_, _ = s.db.Exec(
				`INSERT INTO kpi_revision (symbol, bar_open, provider, algo, body, origin, spec) VALUES (?, ?, ?, ?, ?, ?, ?)
				 ON CONFLICT(symbol, bar_open, algo) DO UPDATE SET body = excluded.body, origin = excluded.origin, spec = excluded.spec`,
				symbol, barOpen, dataSource.String(), Algo, body, origin, AlgoSpec(dataSource),
			)
			return nil
		}
		if strings.Contains(err.Error(), "UNIQUE") {
			return ErrExists
		}
		last = err
	}
	return last
}

// Upsert replaces the KPI body for a symbol and bar open.
func (s *Store) Upsert(symbol string, barOpen int64, body string) error {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		_, err := s.db.Exec(
			`INSERT INTO kpi (symbol, bar_open, body) VALUES (?, ?, ?)
			 ON CONFLICT(symbol, bar_open) DO UPDATE SET body = excluded.body`,
			symbol, barOpen, body,
		)
		if err == nil {
			return nil
		}
		last = err
	}
	return last
}

// Row is one KPI body ready to store.
type Row struct {
	Symbol     string
	BarOpen    int64
	Body       string
	Source     string
	Algo       string
	DataSource shared.DataSource
}

// UpsertAll writes every row in one transaction.
func (s *Store) UpsertAll(rows []Row) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(
		`INSERT INTO kpi (symbol, bar_open, body, source) VALUES (?, ?, ?, ?)
		 ON CONFLICT(symbol, bar_open) DO UPDATE SET body = excluded.body`,
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	rev, err := tx.Prepare(
		`INSERT INTO kpi_revision (symbol, bar_open, provider, algo, body, origin, spec) VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(symbol, bar_open, algo) DO UPDATE SET body = excluded.body, origin = excluded.origin, spec = excluded.spec`,
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer rev.Close()
	for _, row := range rows {
		if _, err := stmt.Exec(row.Symbol, row.BarOpen, row.Body, row.Source); err != nil {
			tx.Rollback()
			return err
		}
		algo := rowAlgo(row)
		dataSource := row.DataSource
		if dataSource == 0 {
			dataSource = shared.DefaultDataSource()
		}
		if _, err := rev.Exec(row.Symbol, row.BarOpen, dataSource.String(), algo, row.Body, row.Source, AlgoSpec(dataSource)); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func rowAlgo(row Row) string {
	if row.Algo != "" {
		return row.Algo
	}
	return Algo
}
