package pgcatalog

import (
	"context"

	"github.com/MaxBernstetter/dagobert/src/calculation-performance"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	"github.com/google/uuid"
)

// LoadCompletedResults rebuilds one Result per completed instance from stored facts.
func (c *Catalog) LoadCompletedResults(ctx context.Context, experimentID string) ([]performance.Result, error) {
	eid, err := parseExperimentID(experimentID)
	if err != nil {
		return nil, err
	}
	trades, err := c.q.ListTradeOutcomesForExperiment(ctx, eid)
	if err != nil {
		return nil, err
	}
	periods, err := c.q.ListCompletedPeriodStats(ctx, eid)
	if err != nil {
		return nil, err
	}
	type acc struct {
		trades  []performance.ClosedTrade
		monthly []performance.PeriodCount
		yearly  []performance.PeriodCount
	}
	order := make([]string, 0)
	byID := map[string]*acc{}
	ensure := func(id string) *acc {
		if row, ok := byID[id]; ok {
			return row
		}
		row := &acc{}
		byID[id] = row
		order = append(order, id)
		return row
	}
	for _, tr := range trades {
		row := ensure(tr.InstanceID.String())
		ct := performance.ClosedTrade{Quote: float64(tr.Quote), Side: int(tr.Side)}
		if tr.Percent.Valid {
			p := float64(tr.Percent.Float32)
			ct.Percent = &p
		}
		row.trades = append(row.trades, ct)
	}
	for _, p := range periods {
		if !p.WinCount.Valid || !p.TradeCount.Valid || p.TradeCount.Int32 <= 0 {
			continue
		}
		row := ensure(p.InstanceID.String())
		bucket := performance.PeriodCount{Period: p.PeriodKey, Trades: int(p.TradeCount.Int32)}
		if p.WinCount.Valid {
			bucket.Wins = int(p.WinCount.Int32)
		}
		switch p.PeriodKind {
		case "year":
			row.yearly = append(row.yearly, bucket)
		default:
			row.monthly = append(row.monthly, bucket)
		}
	}
	out := make([]performance.Result, 0, len(order))
	for _, id := range order {
		row := byID[id]
		out = append(out, performance.Assemble(row.trades, row.monthly, row.yearly))
	}
	return out, nil
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
