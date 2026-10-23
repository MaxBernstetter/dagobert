package store

import (
	"encoding/json"
)

type OrderResultRow struct {
	OrderLinkID string
	Kind        string
	Body        any
}

// ReplaceSimulatedRun replaces order_result and fill_journal for a batch simulated backtest run.
func (s *DB) ReplaceSimulatedRun(orders []OrderResultRow, journals []Journal, balances []BalanceRow) error {
	tx, err := s.sql.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM order_result`); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`DELETE FROM fill_journal`); err != nil {
		tx.Rollback()
		return err
	}
	ordStmt, err := tx.Prepare(`INSERT INTO order_result (order_link_id, kind, body) VALUES (?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer ordStmt.Close()
	for _, row := range orders {
		raw, err := json.Marshal(row.Body)
		if err != nil {
			tx.Rollback()
			return err
		}
		if _, err := ordStmt.Exec(row.OrderLinkID, row.Kind, string(raw)); err != nil {
			tx.Rollback()
			return err
		}
	}
	jStmt, err := tx.Prepare(`INSERT INTO fill_journal (
		order_link_id, venue, venue_symbol, side, base_qty, quote_qty, price, fee, fee_currency, exchange_order_id, price_source
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer jStmt.Close()
	for _, j := range journals {
		if _, err := jStmt.Exec(
			j.OrderLinkID, j.Venue, j.VenueSymbol, j.Side, j.BaseQty, j.QuoteQty, j.Price, j.Fee, j.FeeCurrency, j.ExchangeOrderID, j.PriceSource,
		); err != nil {
			tx.Rollback()
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM balance`); err != nil {
		tx.Rollback()
		return err
	}
	for _, row := range balances {
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
