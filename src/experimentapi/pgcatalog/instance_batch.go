package pgcatalog

import (
	"context"

	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// InstanceBatchItem is the final state of one instance: status, timing and aggregated metrics.
// Everything row-shaped (fills, trade outcomes, period stats) lives in the shard Parquet files;
// ShardKey locates them and is stored only for completed instances.
type InstanceBatchItem struct {
	store.InstanceBatchItem
	Symbol   string
	ShardKey string
}

const updateInstancesBatchSQL = `
UPDATE experiment_instance e SET
  status = t.status, error = t.error, shard_key = t.shard, started_at_ms = t.started, finished_at_ms = t.finished,
  metric_duration_ms = t.m01, metric_cagr = t.m02, metric_max_drawdown = t.m03, metric_sharpe = t.m04,
  metric_sortino = t.m05, metric_monthly_return_avg = t.m06, metric_monthly_return_med = t.m07,
  metric_yearly_return_avg = t.m08, metric_yearly_return_med = t.m09,
  metric_trades_per_month_avg = t.m10, metric_trades_per_month_med = t.m11,
  metric_wins = t.m12, metric_losses = t.m13, metric_breakeven = t.m14,
  metric_monthly_win_rate_avg = t.m15, metric_monthly_win_rate_med = t.m16,
  metric_yearly_win_rate_avg = t.m17, metric_yearly_win_rate_med = t.m18
FROM unnest(
  $2::uuid[], $3::text[], $4::text[], $5::bigint[], $6::bigint[], $7::bigint[],
  $8::float8[], $9::float8[], $10::float8[], $11::float8[], $12::float8[], $13::float8[], $14::float8[],
  $15::float8[], $16::float8[], $17::float8[], $18::int[], $19::int[], $20::int[],
  $21::real[], $22::real[], $23::real[], $24::real[], $25::text[]
) AS t(id, status, error, started, finished, m01, m02, m03, m04, m05, m06, m07, m08, m09, m10, m11,
       m12, m13, m14, m15, m16, m17, m18, shard)
WHERE e.experiment_id = $1 AND e.id = t.id`

const updateProgressSQL = `
UPDATE experiment SET progress_stage = $2, progress_stages = $3, progress_stage_label = $4,
  progress_instance_done = GREATEST(progress_instance_done, $5), progress_instance_total = $6
WHERE id = $1`

func nz(ms int64) *int64 {
	if ms == 0 {
		return nil
	}
	return &ms
}

func f32(p *float64) *float32 {
	if p == nil {
		return nil
	}
	v := float32(*p)
	return &v
}

// PersistInstanceBatch commits many instances in one transaction: sets status, timing and metrics together, and updates the experiment progress counter.
func (c *Catalog) PersistInstanceBatch(ctx context.Context, experimentID string, items []InstanceBatchItem, progressDone, progressTotal int) error {
	sid, err := parseExperimentID(experimentID)
	if err != nil {
		return err
	}
	n := len(items)
	ids := make([]uuid.UUID, n)
	for i, it := range items {
		if ids[i], err = parseInstanceID(it.InstanceID); err != nil {
			return err
		}
	}
	tx, err := c.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	status := make([]string, n)
	errs := make([]string, n)
	shards := make([]string, n)
	started := make([]*int64, n)
	finished := make([]*int64, n)
	f8 := make([][]*float64, 10)
	i4 := make([][]*int32, 3)
	f4 := make([][]*float32, 4)
	for i := range f8 {
		f8[i] = make([]*float64, n)
	}
	for i := range i4 {
		i4[i] = make([]*int32, n)
	}
	for i := range f4 {
		f4[i] = make([]*float32, n)
	}
	for i, it := range items {
		started[i], finished[i] = nz(it.Timing.StartedAtMs), nz(it.Timing.FinishedAtMs)
		if it.Error != "" {
			status[i], errs[i] = store.StatusFailed, it.Error
			continue
		}
		status[i], shards[i] = store.StatusCompleted, it.ShardKey
		m := it.Metrics
		f8[0][i], f8[1][i], f8[2][i], f8[3][i] = m.Cagr, m.MaxDrawdown, m.Sharpe, m.Sortino
		f8[4][i], f8[5][i], f8[6][i], f8[7][i] = m.MonthlyReturnAvg, m.MonthlyReturnMed, m.YearlyReturnAvg, m.YearlyReturnMed
		f8[8][i], f8[9][i] = m.TradesPerMonthAvg, m.TradesPerMonthMed
		i4[0][i], i4[1][i], i4[2][i] = m.Wins, m.Losses, m.Breakeven
		f4[0][i], f4[1][i], f4[2][i], f4[3][i] = f32(m.MonthlyWinRateAvg), f32(m.MonthlyWinRateMed), f32(m.YearlyWinRateAvg), f32(m.YearlyWinRateMed)
	}
	dur := make([]*int64, n)
	for i, it := range items {
		dur[i] = it.Metrics.DurationMs
	}
	args := []any{sid, ids, status, errs, started, finished, dur}
	for _, col := range f8 {
		args = append(args, col)
	}
	for _, col := range i4 {
		args = append(args, col)
	}
	for _, col := range f4 {
		args = append(args, col)
	}
	args = append(args, shards)
	if _, err := tx.Exec(ctx, updateInstancesBatchSQL, args...); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, updateProgressSQL, sid, int32(4), int32(store.ExperimentStageTotal),
		store.ExperimentStageLabelSimulation, int32(progressDone), int32(progressTotal)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func ptrAny[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}
