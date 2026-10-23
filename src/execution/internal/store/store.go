package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"

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
CREATE TABLE IF NOT EXISTS fill_journal (
  order_link_id TEXT PRIMARY KEY,
  venue TEXT NOT NULL,
  venue_symbol TEXT NOT NULL,
  side TEXT NOT NULL,
  base_qty TEXT NOT NULL,
  quote_qty TEXT NOT NULL,
  price TEXT NOT NULL,
  fee TEXT NOT NULL,
  fee_currency TEXT NOT NULL,
  exchange_order_id TEXT NOT NULL,
  price_source TEXT NOT NULL
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

type Journal struct {
	OrderLinkID     string
	Venue           string
	VenueSymbol     string
	Side            string
	BaseQty         string
	QuoteQty        string
	Price           string
	Fee             string
	FeeCurrency     string
	ExchangeOrderID string
	PriceSource     string
}

type DB struct {
	sql *sql.DB
}

func Open(dataDir string) (*DB, error) {
	path := filepath.Join(dataDir, "execution.sqlite")
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		db, err := sql.Open("sqlite", path)
		if err != nil {
			last = err
			continue
		}
		db.SetMaxOpenConns(1)
		if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
			db.Close()
			last = err
			continue
		}
		if _, err := db.Exec(schemaSQL); err != nil {
			db.Close()
			last = err
			continue
		}
		if err := backfillJournals(db); err != nil {
			db.Close()
			last = err
			continue
		}
		return &DB{sql: db}, nil
	}
	return nil, last
}

func (s *DB) Close() error {
	if s.sql == nil {
		return nil
	}
	return s.sql.Close()
}

func backfillJournals(db *sql.DB) error {
	rows, err := db.Query(`SELECT body FROM order_result WHERE kind = 'fill'`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var bodies []string
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return err
		}
		bodies = append(bodies, raw)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, raw := range bodies {
		var body struct {
			OrderLinkID  string `json:"orderLinkId"`
			VenueSymbol  string `json:"venueSymbol"`
			Side         string `json:"side"`
			CumExecQty   string `json:"cumExecQty"`
			CumExecValue string `json:"cumExecValue"`
			AvgPrice     string `json:"avgPrice"`
			CumExecFee   string `json:"cumExecFee"`
			FeeCurrency  string `json:"feeCurrency"`
			OrderID      string `json:"orderId"`
		}
		if err := json.Unmarshal([]byte(raw), &body); err != nil {
			return err
		}
		feeCurrency := body.FeeCurrency
		if feeCurrency == "" && strings.HasSuffix(body.VenueSymbol, "USDC") {
			feeCurrency = "USDC"
		}
		_, err := db.Exec(`INSERT INTO fill_journal (
			order_link_id, venue, venue_symbol, side, base_qty, quote_qty, price, fee, fee_currency, exchange_order_id, price_source
		) VALUES (?, 'bybit', ?, ?, ?, ?, ?, ?, ?, ?, 'binance')
		ON CONFLICT(order_link_id) DO NOTHING`,
			body.OrderLinkID, body.VenueSymbol, body.Side, body.CumExecQty, body.CumExecValue, body.AvgPrice, body.CumExecFee, feeCurrency, body.OrderID,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *DB) UpsertJournal(j Journal) error {
	_, err := s.sql.Exec(`INSERT INTO fill_journal (
		order_link_id, venue, venue_symbol, side, base_qty, quote_qty, price, fee, fee_currency, exchange_order_id, price_source
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(order_link_id) DO UPDATE SET
		base_qty = excluded.base_qty,
		quote_qty = excluded.quote_qty,
		price = excluded.price,
		fee = excluded.fee,
		fee_currency = excluded.fee_currency`,
		j.OrderLinkID, j.Venue, j.VenueSymbol, j.Side, j.BaseQty, j.QuoteQty, j.Price, j.Fee, j.FeeCurrency, j.ExchangeOrderID, j.PriceSource,
	)
	return err
}

func (s *DB) GetOrderResult(orderLinkID string) (OrderResult, error) {
	var r OrderResult
	var body string
	err := s.sql.QueryRow(
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
	_, err = s.sql.Exec(
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
	_, err = s.sql.Exec(
		`UPDATE order_result SET kind = ?, body = ? WHERE order_link_id = ?`,
		kind, string(bodyJSON), orderLinkID,
	)
	return err
}

func (s *DB) ReplaceBalances(rows []BalanceRow) error {
	tx, err := s.sql.Begin()
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
