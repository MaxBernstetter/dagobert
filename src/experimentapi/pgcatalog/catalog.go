package pgcatalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrExperimentCancelled is returned when a phase tries to write its status to an experiment that was
// cancelled. The cancel wins: the phase must stop and the row stays cancelled.
var ErrExperimentCancelled = errors.New("experiment cancelled")

// Catalog is the Postgres experiment catalog for backtest job phases.
type Catalog struct {
	q    *experimentdb.Queries
	pool *pgxpool.Pool
}

// Open connects when DATABASE_URL is set; otherwise returns nil, nil.
func Open(ctx context.Context) (*Catalog, error) {
	url := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if url == "" {
		return nil, nil
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	return &Catalog{q: experimentdb.New(pool), pool: pool}, nil
}

// NewFromPool wraps an existing pool, for callers that own it (the Experiment API Lambda and its backfill server).
func NewFromPool(pool *pgxpool.Pool) *Catalog {
	return &Catalog{q: experimentdb.New(pool), pool: pool}
}

func (c *Catalog) Close() {
	if c != nil && c.pool != nil {
		c.pool.Close()
	}
}

func parseExperimentID(id string) (uuid.UUID, error) {
	return uuid.Parse(strings.TrimSpace(id))
}

func parseInstanceID(id string) (uuid.UUID, error) {
	return uuid.Parse(strings.TrimSpace(id))
}

// GetExperiment loads experiment + instances from Postgres.
func (c *Catalog) GetExperiment(ctx context.Context, id string) (store.ExperimentRecord, error) {
	sid, err := parseExperimentID(id)
	if err != nil {
		return store.ExperimentRecord{}, fmt.Errorf("invalid experiment id: %w", err)
	}
	row, err := c.q.GetExperiment(ctx, sid)
	if err != nil {
		return store.ExperimentRecord{}, err
	}
	insts, err := c.q.ListInstancesByExperiment(ctx, sid)
	if err != nil {
		return store.ExperimentRecord{}, err
	}
	return experimentRecordFromRow(row, insts), nil
}

// ListWorkInstances returns instances to run (all or retry subset).
func (c *Catalog) ListWorkInstances(ctx context.Context, experimentID string, onlyIDs []string, retryFailed bool) (store.ExperimentRecord, []store.ExperimentInstance, error) {
	rec, err := c.GetExperiment(ctx, experimentID)
	if err != nil {
		return store.ExperimentRecord{}, nil, err
	}
	if len(rec.Instances) == 0 {
		return rec, nil, fmt.Errorf("experiment has no instances")
	}
	var work []store.ExperimentInstance
	if len(onlyIDs) > 0 {
		sid, err := parseExperimentID(experimentID)
		if err != nil {
			return rec, nil, err
		}
		ids := make([]uuid.UUID, 0, len(onlyIDs))
		for _, raw := range onlyIDs {
			iid, err := parseInstanceID(raw)
			if err != nil {
				return rec, nil, fmt.Errorf("invalid instance id %s: %w", raw, err)
			}
			ids = append(ids, iid)
		}
		rows, err := c.q.ListInstancesByIDs(ctx, experimentdb.ListInstancesByIDsParams{
			ExperimentID: sid,
			Column2: ids,
		})
		if err != nil {
			return rec, nil, err
		}
		for _, row := range rows {
			work = append(work, instanceFromRow(row))
		}
		return rec, work, nil
	}
	if retryFailed {
		for _, inst := range rec.Instances {
			switch inst.Status {
			case store.StatusFailed, store.StatusRunning:
				work = append(work, inst)
			}
		}
		if len(work) == 0 {
			return rec, nil, fmt.Errorf("no failed instances to retry")
		}
	} else {
		work = rec.Instances
	}
	return rec, work, nil
}

func (c *Catalog) MarkExperimentRunning(ctx context.Context, experimentID string) error {
	sid, err := parseExperimentID(experimentID)
	if err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit
	q := c.q.WithTx(tx)
	n, err := q.MarkExperimentRunningUnlessCancelled(ctx, experimentdb.MarkExperimentRunningUnlessCancelledParams{
		ID:          sid,
		StartedAtMs: pgtype.Int8{Int64: now, Valid: true},
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrExperimentCancelled
	}
	if err := q.MarkInstancesRunning(ctx, sid); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (c *Catalog) MarkExperimentRetryFailed(ctx context.Context, experimentID string) (int, error) {
	sid, err := parseExperimentID(experimentID)
	if err != nil {
		return 0, err
	}
	n, err := c.q.CountFailedInstancesByExperiment(ctx, sid)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, fmt.Errorf("no failed instances to retry")
	}
	now := time.Now().UnixMilli()
	rows, err := c.q.MarkExperimentRetryFailed(ctx, experimentdb.MarkExperimentRetryFailedParams{
		ID:                    sid,
		StartedAtMs:           pgtype.Int8{Int64: now, Valid: true},
		ProgressInstanceTotal: int32(n),
	})
	if err != nil {
		return 0, err
	}
	if rows == 0 {
		return 0, ErrExperimentCancelled
	}
	if err := c.q.MarkFailedInstancesRunning(ctx, sid); err != nil {
		return 0, err
	}
	return int(n), nil
}

func (c *Catalog) MarkExperimentRunningKeepCompleted(ctx context.Context, experimentID string, retryTotal int) error {
	sid, err := parseExperimentID(experimentID)
	if err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	rows, err := c.q.MarkExperimentRunningKeep(ctx, experimentdb.MarkExperimentRunningKeepParams{
		ID:                    sid,
		StartedAtMs:           pgtype.Int8{Int64: now, Valid: true},
		ProgressInstanceTotal: int32(retryTotal),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrExperimentCancelled
	}
	return nil
}

func (c *Catalog) SetExperimentProgress(ctx context.Context, experimentID string, stage int, label string, instanceDone, instanceTotal int) error {
	if stage < 1 {
		stage = 1
	}
	if stage > store.ExperimentStageTotal {
		stage = store.ExperimentStageTotal
	}
	if label == "" {
		label = store.ExperimentStageLabelPrepare
	}
	sid, err := parseExperimentID(experimentID)
	if err != nil {
		return err
	}
	return c.q.UpdateExperimentProgress(ctx, experimentdb.UpdateExperimentProgressParams{
		ID:                    sid,
		ProgressStage:         int32(stage),
		ProgressStages:        int32(store.ExperimentStageTotal),
		ProgressStageLabel:    label,
		ProgressInstanceDone:  int32(instanceDone),
		ProgressInstanceTotal: int32(instanceTotal),
	})
}

func (c *Catalog) MarkExperimentFailed(ctx context.Context, experimentID, message string) error {
	sid, err := parseExperimentID(experimentID)
	if err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	return c.q.MarkExperimentFailed(ctx, experimentdb.MarkExperimentFailedParams{
		ID:           sid,
		Error:        message,
		FinishedAtMs: pgtype.Int8{Int64: now, Valid: true},
	})
}

func (c *Catalog) MarkExperimentCompleted(ctx context.Context, experimentID string) error {
	sid, err := parseExperimentID(experimentID)
	if err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	return c.q.MarkExperimentCompleted(ctx, experimentdb.MarkExperimentCompletedParams{
		ID:           sid,
		FinishedAtMs: pgtype.Int8{Int64: now, Valid: true},
	})
}

func (c *Catalog) MarkExperimentPartial(ctx context.Context, experimentID, message string) error {
	sid, err := parseExperimentID(experimentID)
	if err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	return c.q.MarkExperimentPartial(ctx, experimentdb.MarkExperimentPartialParams{
		ID:           sid,
		Error:        message,
		FinishedAtMs: pgtype.Int8{Int64: now, Valid: true},
	})
}

func (c *Catalog) MarkInstanceCompleted(ctx context.Context, experimentID, instanceID string, timing store.InstanceRunTiming) error {
	sid, err := parseExperimentID(experimentID)
	if err != nil {
		return err
	}
	iid, err := parseInstanceID(instanceID)
	if err != nil {
		return err
	}
	return c.q.UpdateInstanceCompleted(ctx, experimentdb.UpdateInstanceCompletedParams{
		ExperimentID:       sid,
		ID:            iid,
		StartedAtMs:   timingInt8(timing.StartedAtMs),
		FinishedAtMs:  timingInt8(timing.FinishedAtMs),
	})
}

func (c *Catalog) MarkInstanceFailed(ctx context.Context, experimentID, instanceID, message string, timing store.InstanceRunTiming) error {
	sid, err := parseExperimentID(experimentID)
	if err != nil {
		return err
	}
	iid, err := parseInstanceID(instanceID)
	if err != nil {
		return err
	}
	return c.q.UpdateInstanceFailed(ctx, experimentdb.UpdateInstanceFailedParams{
		ExperimentID:      sid,
		ID:           iid,
		Error:        message,
		StartedAtMs:  timingInt8(timing.StartedAtMs),
		FinishedAtMs: timingInt8(timing.FinishedAtMs),
	})
}

func (c *Catalog) UpdateInstanceMetrics(ctx context.Context, experimentID, instanceID string, m store.InstanceListMetrics) error {
	sid, err := parseExperimentID(experimentID)
	if err != nil {
		return err
	}
	iid, err := parseInstanceID(instanceID)
	if err != nil {
		return err
	}
	p := UpdateInstanceMetricsParams(sid, iid, m)
	return c.q.UpdateInstanceMetrics(ctx, p)
}
