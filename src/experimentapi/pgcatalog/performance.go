package pgcatalog

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"sync"

	"github.com/MaxBernstetter/dagobert/src/calculation-performance"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	"github.com/MaxBernstetter/dagobert/src/internal/instanceparquet"
	"github.com/google/uuid"
)

// outcomeReadWorkers bounds the concurrent reads of the per-instance trade outcome files.
const outcomeReadWorkers = 8

// LoadCompletedResults rebuilds one Result per completed instance. Trade outcomes come from each
// instance's trade_outcomes.parquet under dataDir (a missing file is no trades); period stats come
// from Postgres. Instances with neither trades nor period stats are left out.
func (c *Catalog) LoadCompletedResults(ctx context.Context, experimentID, dataDir string) ([]performance.Result, error) {
	eid, err := parseExperimentID(experimentID)
	if err != nil {
		return nil, err
	}
	insts, err := c.q.ListInstancesByExperiment(ctx, eid)
	if err != nil {
		return nil, err
	}
	periods, err := c.q.ListCompletedPeriodStats(ctx, eid)
	if err != nil {
		return nil, err
	}
	type acc struct {
		monthly []performance.PeriodCount
		yearly  []performance.PeriodCount
	}
	periodsByInst := map[string]*acc{}
	for _, p := range periods {
		if !p.WinCount.Valid || !p.TradeCount.Valid || p.TradeCount.Int32 <= 0 {
			continue
		}
		key := p.InstanceID.String()
		row := periodsByInst[key]
		if row == nil {
			row = &acc{}
			periodsByInst[key] = row
		}
		bucket := performance.PeriodCount{Period: p.PeriodKey, Trades: int(p.TradeCount.Int32), Wins: int(p.WinCount.Int32)}
		switch p.PeriodKind {
		case "year":
			row.yearly = append(row.yearly, bucket)
		default:
			row.monthly = append(row.monthly, bucket)
		}
	}

	completed := make([]experimentdb.ExperimentInstance, 0, len(insts))
	for _, inst := range insts {
		if inst.Status == "completed" {
			completed = append(completed, inst)
		}
	}
	trades := make([][]performance.ClosedTrade, len(completed))
	errs := make([]error, len(completed))
	sem := make(chan struct{}, outcomeReadWorkers)
	var wg sync.WaitGroup
	for i := range completed {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			trades[i], errs[i] = instanceTrades(completed[i], dataDir)
		}(i)
	}
	wg.Wait()

	out := make([]performance.Result, 0, len(completed))
	for i, inst := range completed {
		if errs[i] != nil {
			return nil, errs[i]
		}
		row := periodsByInst[inst.ID.String()]
		if len(trades[i]) == 0 && row == nil {
			continue
		}
		var monthly, yearly []performance.PeriodCount
		if row != nil {
			monthly, yearly = row.monthly, row.yearly
		}
		out = append(out, performance.Assemble(trades[i], monthly, yearly))
	}
	return out, nil
}

// instanceTrades reads the closed trades of one instance from its trade outcome file.
func instanceTrades(inst experimentdb.ExperimentInstance, dataDir string) ([]performance.ClosedTrade, error) {
	outcomes, err := instanceparquet.ReadTradeOutcomes(filepath.Join(dataDir, inst.WorkspaceRelPath))
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
