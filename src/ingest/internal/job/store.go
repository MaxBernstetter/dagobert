package job

import (
	"database/sql"
	"fmt"
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
	if err := migrateStoreSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return &store{db: db}, nil
}

func migrateStoreSchema(db *sql.DB) error {
	// Cutover: old symbol-keyed tables are dropped (operators wipe lake data).
	if err := dropIfLegacySymbolPK(db, "stored"); err != nil {
		return err
	}
	if err := dropIfLegacySymbolPK(db, "unavailable"); err != nil {
		return err
	}
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS stored (
		provider TEXT NOT NULL,
		base TEXT NOT NULL,
		quote TEXT NOT NULL,
		open_time INTEGER NOT NULL,
		PRIMARY KEY (provider, base, quote, open_time)
	)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS unavailable (
		provider TEXT NOT NULL,
		base TEXT NOT NULL,
		quote TEXT NOT NULL,
		open_time INTEGER NOT NULL,
		reason TEXT NOT NULL DEFAULT '',
		confirmed_at INTEGER NOT NULL,
		PRIMARY KEY (provider, base, quote, open_time)
	)`)
	return err
}

func dropIfLegacySymbolPK(db *sql.DB, table string) error {
	var sqlText string
	err := db.QueryRow(
		`SELECT sql FROM sqlite_master WHERE type='table' AND name=?`, table,
	).Scan(&sqlText)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if strings.Contains(sqlText, "PRIMARY KEY (provider, base, quote, open_time)") {
		return nil
	}
	_, err = db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS %s`, table))
	return err
}

func (s *store) close() error {
	return s.db.Close()
}

func (s *store) list(provider, base, quote string, from, to int64) ([]int64, error) {
	rows, err := s.db.Query(
		`SELECT open_time FROM stored WHERE provider = ? AND base = ? AND quote = ? AND open_time >= ? AND open_time < ? ORDER BY open_time`,
		provider, base, quote, from, to,
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

func (s *store) insert(provider, base, quote string, times []int64) error {
	if len(times) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO stored(provider, base, quote, open_time) VALUES (?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, t := range times {
		if _, err := stmt.Exec(provider, base, quote, t); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *store) listUnavailable(provider, base, quote string, from, to int64) ([]int64, error) {
	rows, err := s.db.Query(
		`SELECT open_time FROM unavailable WHERE provider = ? AND base = ? AND quote = ? AND open_time >= ? AND open_time < ? ORDER BY open_time`,
		provider, base, quote, from, to,
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

func (s *store) insertUnavailable(provider, base, quote string, times []int64, reason string, confirmedAt int64) error {
	if len(times) == 0 {
		return nil
	}
	if reason == "" {
		reason = "empty"
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(
		`INSERT OR REPLACE INTO unavailable(provider, base, quote, open_time, reason, confirmed_at) VALUES (?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, t := range times {
		if _, err := stmt.Exec(provider, base, quote, t, reason, confirmedAt); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *store) clearUnavailable(provider, base, quote string, from, to int64) error {
	_, err := s.db.Exec(
		`DELETE FROM unavailable WHERE provider = ? AND base = ? AND quote = ? AND open_time >= ? AND open_time < ?`,
		provider, base, quote, from, to,
	)
	return err
}
