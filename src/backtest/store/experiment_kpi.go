package store

import (
	"fmt"

	tirunner "github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/runner"
)

// UpsertExperimentKPIRows writes shared indicator bodies for one experiment (once at finalize).
func (c *Catalog) UpsertExperimentKPIRows(experimentID string, rows []tirunner.KPIRow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(
		`INSERT INTO experiment_kpi (experiment_id, symbol, bar_open, algo, body, origin, spec)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(experiment_id, symbol, bar_open, algo)
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
		if _, err := stmt.Exec(experimentID, row.Symbol, row.BarOpen, tirunner.KPIAlgo, row.Body, row.Source, spec); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// CountExperimentKPIRows returns how many KPI bars are stored for a experiment.
func (c *Catalog) CountExperimentKPIRows(experimentID string) (int, error) {
	var n int
	err := c.db.QueryRow(`SELECT COUNT(*) FROM experiment_kpi WHERE experiment_id = ?`, experimentID).Scan(&n)
	return n, err
}

// LoadExperimentKPIBodies loads shared KPI JSON bodies for report/chart use.
func (c *Catalog) LoadExperimentKPIBodies(experimentID, symbol, algo string, minOpen, maxOpen int64) (map[int64]string, error) {
	if algo == "" {
		algo = tirunner.KPIAlgo
	}
	rows, err := c.db.Query(
		`SELECT bar_open, body FROM experiment_kpi
		 WHERE experiment_id = ? AND symbol = ? AND algo = ? AND bar_open >= ? AND bar_open <= ?`,
		experimentID, symbol, algo, minOpen, maxOpen,
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
		return nil, fmt.Errorf("no experiment KPI rows")
	}
	return out, nil
}
