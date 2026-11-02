package backtest

import (
	"context"
	"errors"

	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/pgcatalog"
)

// experimentRunCatalog is the experiment catalog a run writes to: Postgres, the only catalog for
// experiments (DATABASE_URL is required).
type experimentRunCatalog struct {
	pg *pgcatalog.Catalog
}

var errNoExperimentDB = errors.New("DATABASE_URL required: the experiment catalog is Postgres")

func openExperimentRunCatalog() (*experimentRunCatalog, error) {
	pg, err := pgcatalog.Open(context.Background())
	if err != nil {
		return nil, err
	}
	if pg == nil {
		return nil, errNoExperimentDB
	}
	return &experimentRunCatalog{pg: pg}, nil
}

func (c *experimentRunCatalog) Close() {
	if c != nil && c.pg != nil {
		c.pg.Close()
	}
}

func (c *experimentRunCatalog) ctx() context.Context {
	return context.Background()
}

func (c *experimentRunCatalog) GetExperiment(experimentID string) (store.ExperimentRecord, error) {
	return c.pg.GetExperiment(c.ctx(), experimentID)
}

func (c *experimentRunCatalog) MarkExperimentFailed(experimentID, message string) error {
	return c.pg.MarkExperimentFailed(c.ctx(), experimentID, message)
}

func (c *experimentRunCatalog) MarkExperimentRunning(experimentID string) error {
	return c.pg.MarkExperimentRunning(c.ctx(), experimentID)
}

func (c *experimentRunCatalog) MarkExperimentRetryFailed(experimentID string) (int, error) {
	return c.pg.MarkExperimentRetryFailed(c.ctx(), experimentID)
}

func (c *experimentRunCatalog) MarkExperimentRunningKeepCompleted(experimentID string, retryTotal int) error {
	return c.pg.MarkExperimentRunningKeepCompleted(c.ctx(), experimentID, retryTotal)
}

func (c *experimentRunCatalog) SetExperimentProgress(experimentID string, stage int, label string, instanceDone, instanceTotal int) error {
	return c.pg.SetExperimentProgress(c.ctx(), experimentID, stage, label, instanceDone, instanceTotal)
}

func (c *experimentRunCatalog) MarkExperimentInstanceFailed(experimentID, instanceID, message string, timing store.InstanceRunTiming) error {
	return c.pg.MarkInstanceFailed(c.ctx(), experimentID, instanceID, message, timing)
}

func (c *experimentRunCatalog) MarkExperimentInstanceCompleted(experimentID, instanceID string, timing store.InstanceRunTiming) error {
	return c.pg.MarkInstanceCompleted(c.ctx(), experimentID, instanceID, timing)
}

func (c *experimentRunCatalog) UpdateExperimentInstanceMetrics(experimentID, instanceID string, m store.InstanceListMetrics) error {
	return c.pg.UpdateInstanceMetrics(c.ctx(), experimentID, instanceID, m)
}

func (c *experimentRunCatalog) MarkExperimentCompleted(experimentID string) error {
	return c.pg.MarkExperimentCompleted(c.ctx(), experimentID)
}

func (c *experimentRunCatalog) MarkExperimentPartial(experimentID, message string) error {
	return c.pg.MarkExperimentPartial(c.ctx(), experimentID, message)
}

// PersistInstanceBatch commits a batch of instance results (and progress) in one transaction.
func (c *experimentRunCatalog) PersistInstanceBatch(experimentID string, results []instanceResult, progressDone, progressTotal int) error {
	items := make([]pgcatalog.InstanceBatchItem, len(results))
	for i, r := range results {
		items[i] = r.pgItem()
	}
	return c.pg.PersistInstanceBatch(c.ctx(), experimentID, items, progressDone, progressTotal)
}
