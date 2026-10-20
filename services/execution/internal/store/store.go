package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const (
	KindFill      = "fill"
	KindRejection = "rejection"
	KindNoTrade   = "no-trade"
	KindPending   = "pending"
)

var ErrNotFound = errors.New("order result not found")

const schemaSQL = `
CREATE TABLE IF NOT EXISTS order_result (
  order_link_id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  body TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS balance (
  currency TEXT PRIMARY KEY,
  cash_balance TEXT NOT NULL,
  transaction_time INTEGER NOT NULL
);
`

type OrderResult struct {
	OrderLinkID string
	Kind        string
	Body        json.RawMessage
}

type BalanceRow struct {
	Currency        string
	CashBalance     string
	TransactionTime int64
}

type DB struct {
	path string
}

func Open(dataDir string) (*DB, error) {
	path := filepath.Join(dataDir, "execution.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, err
	}
	db.Close()
	return &DB{path: path}, nil
}

func (s *DB) GetOrderResult(orderLinkID string) (OrderResult, error) {
	db, err := sql.Open("sqlite", s.path)
	if err != nil {
		return OrderResult{}, err
	}
	defer db.Close()

	var r OrderResult
	var body string
	err = db.QueryRow(
		`SELECT order_link_id, kind, body FROM order_result WHERE order_link_id = ?`,
		orderLinkID,
	).Scan(&r.OrderLinkID, &r.Kind, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return OrderResult{}, ErrNotFound
	}
	if err != nil {
		return OrderResult{}, err
	}
	r.Body = json.RawMessage(body)
	return r, nil
}

func (s *DB) InsertOrderResult(orderLinkID, kind string, body any) error {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}
	db, err := sql.Open("sqlite", s.path)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(
		`INSERT INTO order_result (order_link_id, kind, body) VALUES (?, ?, ?)`,
		orderLinkID, kind, string(bodyJSON),
	)
	return err
}

func (s *DB) UpdateOrderResult(orderLinkID, kind string, body any) error {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}
	db, err := sql.Open("sqlite", s.path)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(
		`UPDATE order_result SET kind = ?, body = ? WHERE order_link_id = ?`,
		kind, string(bodyJSON), orderLinkID,
	)
	return err
}

func (s *DB) ReplaceBalances(rows []BalanceRow) error {
	db, err := sql.Open("sqlite", s.path)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM balance`); err != nil {
		tx.Rollback()
		return err
	}
	for _, row := range rows {
		if _, err := tx.Exec(
			`INSERT INTO balance (currency, cash_balance, transaction_time) VALUES (?, ?, ?)`,
			row.Currency, row.CashBalance, row.TransactionTime,
		); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
