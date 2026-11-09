package pgcatalog

import (
	"context"
	"errors"
	"io/fs"
	"sync"

	"github.com/MaxBernstetter/dagobert/src/calculation-performance"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	"github.com/MaxBernstetter/dagobert/src/internal/instanceparquet"
	"github.com/google/uuid"
)

// outcomeReadWorkers bounds the concurrent reads of the per-instance trade outcome row groups.
const outcomeReadWorkers = 8

// LoadCompletedResults rebuilds one Result per completed instance from its row groups in the shard
// files trade_outcomes.parquet and period_stats.parquet under <shard_key> (read through the workspace store) (an instance
// without shard key or row group is no trades, no periods). Instances with neither are left out.
func (c *Catalog) LoadCompletedResults(ctx context.Context, experimentID, dataDir string) ([]performance.Result, error) {
	eid, err := parseExperimentID(experimentID)
	if err != nil {
		return nil, err
	}
	insts, err := c.q.ListInstancesByExperiment(ctx, eid)
	if err != nil {
		return nil, err
	}
	completed := make([]experimentdb.ExperimentInstance, 0, len(insts))
	for _, inst := range insts {
		if inst.Status == "completed" {
			completed = append(completed, inst)
		}
	}
	type loaded struct {
		trades          []performance.ClosedTrade
		monthly, yearly []performance.PeriodCount
		err             error
	}
	res := make([]loaded, len(completed))
	sem := make(chan struct{}, outcomeReadWorkers)
	var wg sync.WaitGroup
	for i := range completed {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			if completed[i].ShardKey == "" {
				return
			}
			dir := completed[i].ShardKey
			id := completed[i].ID.String()
			r := &res[i]
			if r.trades, r.err = InstanceTrades(ctx, dataDir, dir, id); r.err != nil {
				return
			}
			r.monthly, r.yearly, r.err = PeriodCounts(ctx, dataDir, dir, id)
		}(i)
	}
	wg.Wait()

	out := make([]performance.Result, 0, len(completed))
	for _, r := range res {
		if r.err != nil {
			return nil, r.err
		}
		if len(r.trades) == 0 && len(r.monthly) == 0 && len(r.yearly) == 0 {
			continue
		}
		out = append(out, performance.Assemble(r.trades, r.monthly, r.yearly))
	}
	return out, nil
}

// InstanceTrades reads the closed trades of one instance from its trade outcome row group. A missing
// file or row group is no trades.
func InstanceTrades(ctx context.Context, dataDir, shardKey, instanceID string) ([]performance.ClosedTrade, error) {
	outcomes, err := instanceparquet.ReadTradeOutcomes(ctx, dataDir, shardKey, instanceID)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	closed := make([]performance.ClosedTrade, 0, len(outcomes))
	for _, tr := range outcomes {
		ct := performance.ClosedTrade{Quote: float64(tr.Quote), Side: int(tr.Side)}
		if tr.Percent != nil {
			p := float64(*tr.Percent)
			ct.Percent = &p
		}
		closed = append(closed, ct)
	}
	return closed, nil
}

// PeriodCounts reads the month and year buckets with at least one trade from the instance's row group
// in <shard>/period_stats.parquet. A missing file or row group is no buckets.
func PeriodCounts(ctx context.Context, dataDir, shardKey, instanceID string) (monthly, yearly []performance.PeriodCount, err error) {
	rows, err := instanceparquet.ReadPeriodStats(ctx, dataDir, shardKey, instanceID)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	for _, p := range rows {
		if p.WinCount == nil || p.TradeCount == nil || *p.TradeCount <= 0 {
			continue
		}
		bucket := performance.PeriodCount{Period: p.Key, Trades: int(*p.TradeCount), Wins: int(*p.WinCount)}
		if p.Kind == "year" {
			yearly = append(yearly, bucket)
		} else {
			monthly = append(monthly, bucket)
		}
	}
	return monthly, yearly, nil
}

// ReplaceExperimentPerformance writes the experiment reduction. An empty result deletes it.
func (c *Catalog) ReplaceExperimentPerformance(ctx context.Context, experimentID string, result performance.Result) error {
	eid, err := parseExperimentID(experimentID)
	if err != nil {
		return err
	}
	if err := c.q.DeleteExperimentOutcomeStats(ctx, eid); err != nil {
		return err
	}
	if err := c.q.DeleteExperimentPeriodStats(ctx, eid); err != nil {
		return err
	}
	if result.ClosedTrades() == 0 {
		return c.q.DeleteExperimentPerformance(ctx, eid)
	}
	if err := c.q.UpsertExperimentPerformance(ctx, experimentdb.UpsertExperimentPerformanceParams{
		ExperimentID: eid,
		Wins:         int32(result.Wins),
		Losses:       int32(result.Losses),
		Breakeven:    int32(result.Breakeven),
	}); err != nil {
		return err
	}
	for _, bucket := range result.Monthly {
		if err := c.q.InsertExperimentPeriodStat(ctx, experimentdb.InsertExperimentPeriodStatParams{
			ExperimentID: eid,
			PeriodKind:   "month",
			PeriodKey:    bucket.Period,
			Wins:         int32(bucket.Wins),
			Trades:       int32(bucket.Trades),
		}); err != nil {
			return err
		}
	}
	for _, bucket := range result.Yearly {
		if err := c.q.InsertExperimentPeriodStat(ctx, experimentdb.InsertExperimentPeriodStatParams{
			ExperimentID: eid,
			PeriodKind:   "year",
			PeriodKey:    bucket.Period,
			Wins:         int32(bucket.Wins),
			Trades:       int32(bucket.Trades),
		}); err != nil {
			return err
		}
	}
	for _, row := range outcomeRows(eid, result.Gain, performance.SideGain) {
		if err := c.q.InsertExperimentOutcomeStat(ctx, row); err != nil {
			return err
		}
	}
	for _, row := range outcomeRows(eid, result.Loss, performance.SideLoss) {
		if err := c.q.InsertExperimentOutcomeStat(ctx, row); err != nil {
			return err
		}
	}
	return nil
}

func outcomeRows(experimentID uuid.UUID, side *performance.SideStats, sideCode int) []experimentdb.InsertExperimentOutcomeStatParams {
	if side == nil {
		return nil
	}
	var out []experimentdb.InsertExperimentOutcomeStatParams
	if side.Absolute != nil {
		out = append(out, outcomeRow(experimentID, sideCode, performance.UnitQuote, side.Absolute))
	}
	if side.Percentage != nil {
		out = append(out, outcomeRow(experimentID, sideCode, performance.UnitPercent, side.Percentage))
	}
	return out
}

func outcomeRow(id uuid.UUID, side, unit int, d *performance.Distribution) experimentdb.InsertExperimentOutcomeStatParams {
	return experimentdb.InsertExperimentOutcomeStatParams{
		ExperimentID: id,
		Side:         int16(side),
		Unit:         int16(unit),
		Average:      float32(d.Average),
		P10:          float32(d.P10),
		P25:          float32(d.P25),
		P50:          float32(d.P50),
		P75:          float32(d.P75),
		P90:          float32(d.P90),
		P95:          float32(d.P95),
	}
}
