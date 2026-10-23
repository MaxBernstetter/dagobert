package signalstore

import (
	"database/sql"
)

type SignalRow struct {
	Playbook    string
	Symbol      string
	CandleClose int64
	Body        []byte
}

type ActionRow struct {
	Playbook    string
	Symbol      string
	CandleClose int64
	Body        []byte
}

// ReplaceSignalsAndActions writes all rows in one transaction (replaces prior rows for same keys).
func ReplaceSignalsAndActions(db *sql.DB, signals []SignalRow, actions []ActionRow) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	sigStmt, err := tx.Prepare(
		`INSERT INTO signal (playbook, symbol, candle_close, body) VALUES (?, ?, ?, ?)
		 ON CONFLICT(playbook, symbol, candle_close) DO UPDATE SET body = excluded.body`,
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer sigStmt.Close()
	actStmt, err := tx.Prepare(
		`INSERT INTO action (playbook, symbol, candle_close, body) VALUES (?, ?, ?, ?)
		 ON CONFLICT(playbook, symbol, candle_close) DO UPDATE SET body = excluded.body`,
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer actStmt.Close()

	for _, row := range signals {
		if _, err := sigStmt.Exec(row.Playbook, row.Symbol, row.CandleClose, string(row.Body)); err != nil {
			tx.Rollback()
			return err
		}
	}
	for _, row := range actions {
		if _, err := actStmt.Exec(row.Playbook, row.Symbol, row.CandleClose, string(row.Body)); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
