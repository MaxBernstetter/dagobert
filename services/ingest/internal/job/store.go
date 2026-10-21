package job

import (
	"database/sql"
	"net/url"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

type store struct {
	db *sql.DB
}

func openStore(dataDir string) (*store, error) {
	path := filepath.Join(dataDir, "ingest.sqlite")
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	u := url.URL{
		Scheme:   "file",
		Path:     filepath.ToSlash(abs),
		RawQuery: "_pragma=busy_timeout(5000)",
	}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS stored (
		symbol TEXT NOT NULL,
		open_time INTEGER NOT NULL,
		provider TEXT NOT NULL DEFAULT 'binance',
		PRIMARY KEY (symbol, open_time)
	)`)
	if err != nil {
		db.Close()
		return nil, err
	}
	if _, err = db.Exec(`ALTER TABLE stored ADD COLUMN provider TEXT NOT NULL DEFAULT 'binance'`); err != nil && !strings.Contains(err.Error(), "duplicate column") {
		db.Close()
		return nil, err
	}
	return &store{db: db}, nil
}

func (s *store) close() error {
	return s.db.Close()
}

func (s *store) list(symbol string, from, to int64) ([]int64, error) {
	rows, err := s.db.Query(
		`SELECT open_time FROM stored WHERE symbol = ? AND open_time >= ? AND open_time < ? ORDER BY open_time`,
		symbol, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var t int64
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *store) insert(symbol string, times []int64) error {
	if len(times) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO stored(symbol, open_time, provider) VALUES (?, ?, 'binance')`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, t := range times {
		if _, err := stmt.Exec(symbol, t); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
