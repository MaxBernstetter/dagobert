package pgcatalog

import (
	"context"
	"strings"

	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// PeriodStatRow is one bucket for experiment_instance_period_stat.
type PeriodStatRow struct {
	Kind       string
	Key        string
	ReturnPct  *float64
	TradeCount *int32
	WinCount   *int32
}

// FillRow is one fill for experiment_instance_fill.
type FillRow struct {
	TsMs  int64
	Side  string
	Qty   float64
	Price float64
}

// TradeOutcomeRow is one winning or losing sell.
type TradeOutcomeRow struct {
	Side    int16
	Quote   float32
	Percent *float32
}

// ReplaceInstanceActivity replaces period stats and fills for an instance.
func (c *Catalog) ReplaceInstanceActivity(
	ctx context.Context,
	experimentID, instanceID, symbol string,
	stats []PeriodStatRow,
	fills []FillRow,
	outcomes []TradeOutcomeRow,
) error {
	sid, err := parseExperimentID(experimentID)
	if err != nil {
		return err
	}
	iid, err := parseInstanceID(instanceID)
	if err != nil {
		return err
	}
	if err := c.q.DeletePeriodStatsForInstance(ctx, iid); err != nil {
		return err
	}
	for _, p := range stats {
		tc := pgtype.Int4{}
		if p.TradeCount != nil {
			tc = pgtype.Int4{Int32: *p.TradeCount, Valid: true}
		}
		wc := pgtype.Int4{}
		if p.WinCount != nil {
			wc = pgtype.Int4{Int32: *p.WinCount, Valid: true}
		}
		ret := pgtype.Float8{}
		if p.ReturnPct != nil {
			ret = pgtype.Float8{Float64: *p.ReturnPct, Valid: true}
		}
		if err := c.q.UpsertPeriodStat(ctx, experimentdb.UpsertPeriodStatParams{
			InstanceID: iid,
			PeriodKind: p.Kind,
			PeriodKey:  p.Key,
			ReturnPct:  ret,
			TradeCount: tc,
			WinCount:   wc,
		}); err != nil {
			return err
		}
	}
	if err := c.q.DeleteTradeOutcomesForInstance(ctx, iid); err != nil {
		return err
	}
	for i, o := range outcomes {
		pct := pgtype.Float4{}
		if o.Percent != nil {
			pct = pgtype.Float4{Float32: *o.Percent, Valid: true}
		}
		if err := c.q.InsertTradeOutcome(ctx, experimentdb.InsertTradeOutcomeParams{
			InstanceID: iid,
			Ordinal:    int32(i),
			Side:       o.Side,
			Quote:      o.Quote,
			Percent:    pct,
		}); err != nil {
			return err
		}
	}
	if err := c.q.DeleteFillsForInstance(ctx, experimentdb.DeleteFillsForInstanceParams{
		ExperimentID: sid,
		InstanceID:   iid,
	}); err != nil {
		return err
	}
	for _, f := range fills {
		side := strings.ToLower(strings.TrimSpace(f.Side))
		if side == "" {
			continue
		}
		if err := c.q.InsertFill(ctx, experimentdb.InsertFillParams{
			ID:           uuid.New(),
			ExperimentID: sid,
			InstanceID:   iid,
			ActionID:     pgtype.UUID{},
			Symbol:       symbol,
			TsMs:         f.TsMs,
			Side:         side,
			Qty:          f.Qty,
			Price:        f.Price,
			Fee:          0,
			FeeAsset:     "",
		}); err != nil {
			return err
		}
	}
	return nil
}
