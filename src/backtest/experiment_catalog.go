package backtest

import (
	"context"

	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/pgcatalog"
	tirunner "github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/runner"
)

// experimentRunCatalog routes catalog mutations to Postgres (when DATABASE_URL is set) or SQLite.
type experimentRunCatalog struct {
	pg     *pgcatalog.Catalog
	sqlite *store.Catalog
}

func openExperimentRunCatalog(dataDir string) (*experimentRunCatalog, error) {
	ctx := context.Background()
	pg, err := pgcatalog.Open(ctx)
	if err != nil {
		return nil, err
	}
	rc := &experimentRunCatalog{pg: pg}
	if pg == nil {
		cat, err := store.Open(dataDir)
		if err != nil {
			return nil, err
		}
		rc.sqlite = cat
	}
	return rc, nil
}

func (c *experimentRunCatalog) Close() {
	if c == nil {
		return
	}
	if c.pg != nil {
		c.pg.Close()
	}
	if c.sqlite != nil {
		c.sqlite.Close()
	}
}

func (c *experimentRunCatalog) usesPostgres() bool {
	return c != nil && c.pg != nil
}

func (c *experimentRunCatalog) ctx() context.Context {
	return context.Background()
}

func (c *experimentRunCatalog) GetExperiment(experimentID string) (store.ExperimentRecord, error) {
	if c.usesPostgres() {
		return c.pg.GetExperiment(c.ctx(), experimentID)
	}
	return c.sqlite.GetExperiment(experimentID)
}

func (c *experimentRunCatalog) MarkExperimentFailed(experimentID, message string) error {
	if c.usesPostgres() {
		return c.pg.MarkExperimentFailed(c.ctx(), experimentID, message)
	}
	return c.sqlite.MarkExperimentFailed(experimentID, message)
}

func (c *experimentRunCatalog) MarkExperimentRunning(experimentID string) error {
	if c.usesPostgres() {
		return c.pg.MarkExperimentRunning(c.ctx(), experimentID)
	}
	return c.sqlite.MarkExperimentRunning(experimentID)
}

func (c *experimentRunCatalog) MarkExperimentRetryFailed(experimentID string) (int, error) {
	if c.usesPostgres() {
		return c.pg.MarkExperimentRetryFailed(c.ctx(), experimentID)
	}
	return c.sqlite.MarkExperimentRetryFailed(experimentID)
}

func (c *experimentRunCatalog) MarkExperimentRunningKeepCompleted(experimentID string, retryTotal int) error {
	if c.usesPostgres() {
		return c.pg.MarkExperimentRunningKeepCompleted(c.ctx(), experimentID, retryTotal)
	}
	return c.sqlite.MarkExperimentRunningKeepCompleted(experimentID, retryTotal)
}

func (c *experimentRunCatalog) SetExperimentProgress(experimentID string, stage int, label string, instanceDone, instanceTotal int) error {
	if c.usesPostgres() {
		return c.pg.SetExperimentProgress(c.ctx(), experimentID, stage, label, instanceDone, instanceTotal)
	}
	return c.sqlite.SetExperimentProgress(experimentID, stage, label, instanceDone, instanceTotal)
}

func (c *experimentRunCatalog) MarkExperimentInstanceFailed(experimentID, instanceID, message string, timing store.InstanceRunTiming) error {
	if c.usesPostgres() {
		return c.pg.MarkInstanceFailed(c.ctx(), experimentID, instanceID, message, timing)
	}
	return c.sqlite.MarkExperimentInstanceFailed(instanceID, message, timing)
}

func (c *experimentRunCatalog) MarkExperimentInstanceCompleted(experimentID, instanceID string, timing store.InstanceRunTiming) error {
	if c.usesPostgres() {
		return c.pg.MarkInstanceCompleted(c.ctx(), experimentID, instanceID, timing)
	}
	return c.sqlite.MarkExperimentInstanceCompleted(instanceID, timing)
}

func (c *experimentRunCatalog) UpdateExperimentInstanceMetrics(experimentID, instanceID string, m store.InstanceListMetrics) error {
	if c.usesPostgres() {
		return c.pg.UpdateInstanceMetrics(c.ctx(), experimentID, instanceID, m)
	}
	return c.sqlite.UpdateExperimentInstanceMetrics(instanceID, m)
}

func (c *experimentRunCatalog) MarkExperimentCompleted(experimentID string) error {
	if c.usesPostgres() {
		return c.pg.MarkExperimentCompleted(c.ctx(), experimentID)
	}
	return c.sqlite.MarkExperimentCompleted(experimentID)
}

func (c *experimentRunCatalog) MarkExperimentPartial(experimentID, message string) error {
	if c.usesPostgres() {
		return c.pg.MarkExperimentPartial(c.ctx(), experimentID, message)
	}
	return c.sqlite.MarkExperimentPartial(experimentID, message)
}

func (c *experimentRunCatalog) UpsertExperimentKPIRows(experimentID string, rows []tirunner.KPIRow) error {
	if c.sqlite == nil {
		return nil
	}
	return c.sqlite.UpsertExperimentKPIRows(experimentID, rows)
}
