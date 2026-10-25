package store

import (
	"fmt"

	tirunner "github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/runner"
)

// UpsertKPIRows writes indicator bodies for one backtest (never touches prod TI).
func (c *Catalog) UpsertKPIRows(backtestID string, rows []tirunner.KPIRow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(
		`INSERT INTO backtest_kpi (backtest_id, symbol, bar_open, algo, body, origin, spec)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(backtest_id, symbol, bar_open, algo)
		 DO UPDATE SET body = excluded.body, origin = excluded.origin, spec = excluded.spec`,
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, row := range rows {
		ds := row.DataSource
		spec := tirunner.KPIAlgoSpec(ds)
		if _, err := stmt.Exec(backtestID, row.Symbol, row.BarOpen, tirunner.KPIAlgo, row.Body, row.Source, spec); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// ListBarOpens returns bar_open values for replay (caller adds bar duration for candle close).
func (c *Catalog) ListBarOpens(backtestID, symbol, algo string, minOpen, maxOpen int64) ([]int64, error) {
	if algo == "" {
		algo = tirunner.KPIAlgo
	}
	rows, err := c.db.Query(
		`SELECT bar_open FROM backtest_kpi
		 WHERE backtest_id = ? AND symbol = ? AND algo = ? AND bar_open >= ? AND bar_open <= ?
		 ORDER BY bar_open`,
		backtestID, symbol, algo, minOpen, maxOpen,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// LoadKPIBodies loads KPI JSON bodies for kpi.Window loader.
func (c *Catalog) LoadKPIBodies(backtestID, symbol, algo string, minOpen, maxOpen int64) (map[int64]string, error) {
	if algo == "" {
		algo = tirunner.KPIAlgo
	}
	rows, err := c.db.Query(
		`SELECT bar_open, body FROM backtest_kpi
		 WHERE backtest_id = ? AND symbol = ? AND algo = ? AND bar_open >= ? AND bar_open <= ?`,
		backtestID, symbol, algo, minOpen, maxOpen,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64]string)
	for rows.Next() {
		var barOpen int64
		var body string
		if err := rows.Scan(&barOpen, &body); err != nil {
			return nil, err
		}
		out[barOpen] = body
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no backtest KPI rows")
	}
	return out, nil
}
