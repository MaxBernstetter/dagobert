package experimentapi

import (
	"context"
	"errors"
	"io/fs"

	"connectrpc.com/connect"
	"github.com/MaxBernstetter/dagobert/src/calculation-performance"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/pgcatalog"
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/structpb"
)

func winRateDef() *v1.KpiDef {
	return &v1.KpiDef{
		Slug:   "win-rate",
		Title:  "Win rate",
		Label:  "Win rate",
		Kind:   "strategy-kpi",
		Unit:   "ratio",
		Format: "percent",
	}
}

func (s *Server) attachInstancePerformance(ctx context.Context, inst experimentdb.ExperimentInstance, resp *v1.KpiReport) error {
	dir, err := instanceShard(inst)
	if errors.Is(err, fs.ErrNotExist) {
		return nil // no shard: no trades, no periods
	}
	closed, err := pgcatalog.InstanceTrades(ctx, dataDir(), dir, inst.ID.String())
	if err != nil {
		return err
	}
	monthly, yearly, err := pgcatalog.PeriodCounts(ctx, dataDir(), dir, inst.ID.String())
	if err != nil {
		return err
	}
	result := performance.Assemble(closed, monthly, yearly)
	if result.ClosedTrades() == 0 {
		return nil
	}
	kept := resp.Results[:0]
	for _, row := range resp.Results {
		if row.Def.GetSlug() == "win-rate" {
			continue
		}
		kept = append(kept, row)
	}
	resp.Results = kept
	if result.WinRate != nil {
		resp.Results = append(resp.Results, &v1.KpiResult{Def: winRateDef(), Value: structpb.NewNumberValue(*result.WinRate)})
	}
	applyPerformanceStats(resp.Statistics, result)
	return nil
}

func applyPerformanceStats(stats *v1.KpiStatistics, result performance.Result) {
	stats.WinRate = result.WinRate
	if result.MonthlyWinRate != nil {
		stats.MonthlyWinRateAvg = &result.MonthlyWinRate.Average
		stats.MonthlyWinRateMed = &result.MonthlyWinRate.Median
	}
	if result.YearlyWinRate != nil {
		stats.YearlyWinRateAvg = &result.YearlyWinRate.Average
		stats.YearlyWinRateMed = &result.YearlyWinRate.Median
	}
	stats.Gain = sideFrom(result.Gain)
	stats.Loss = sideFrom(result.Loss)
}

func sideFrom(side *performance.SideStats) *v1.OutcomeSide {
	if side == nil {
		return nil
	}
	out := &v1.OutcomeSide{Absolute: distFrom(side.Absolute), Percentage: distFrom(side.Percentage)}
	if out.Absolute == nil && out.Percentage == nil {
		return nil
	}
	return out
}

func distFrom(d *performance.Distribution) *v1.OutcomeDist {
	if d == nil {
		return nil
	}
	return &v1.OutcomeDist{
		Average: d.Average,
		P10:     d.P10,
		P25:     d.P25,
		P50:     d.P50,
		P75:     d.P75,
		P90:     d.P90,
		P95:     d.P95,
	}
}

// GetExperimentPerformance serves the stored experiment reduction.
func (s *Server) GetExperimentPerformance(ctx context.Context, req *connect.Request[v1.GetExperimentPerformanceRequest]) (*connect.Response[v1.GetExperimentPerformanceResponse], error) {
	id, err := parseExperimentID(req.Msg.Id)
	if err != nil {
		return nil, invalidArgument("invalid id")
	}
	row, err := s.q.GetExperimentPerformance(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, notFound("performance not found")
	}
	if err != nil {
		return nil, internal(err)
	}
	periods, err := s.q.ListExperimentPeriodStats(ctx, id)
	if err != nil {
		return nil, internal(err)
	}
	outcomes, err := s.q.ListExperimentOutcomeStats(ctx, id)
	if err != nil {
		return nil, internal(err)
	}
	var monthly, yearly []performance.PeriodCount
	for _, p := range periods {
		bucket := performance.PeriodCount{Period: p.PeriodKey, Wins: int(p.Wins), Trades: int(p.Trades)}
		if p.PeriodKind == "year" {
			yearly = append(yearly, bucket)
		} else {
			monthly = append(monthly, bucket)
		}
	}
	stats := &v1.KpiStatistics{}
	if row.WinRate.Valid {
		rate := float64(row.WinRate.Float32)
		stats.WinRate = &rate
	}
	if rates := performance.PeriodRates(monthly); rates != nil {
		stats.MonthlyWinRateAvg = &rates.Average
		stats.MonthlyWinRateMed = &rates.Median
	}
	if rates := performance.PeriodRates(yearly); rates != nil {
		stats.YearlyWinRateAvg = &rates.Average
		stats.YearlyWinRateMed = &rates.Median
	}
	stats.Gain, stats.Loss = sidesFromRows(outcomes)
	var results []*v1.KpiResult
	if stats.WinRate != nil {
		results = []*v1.KpiResult{{Def: winRateDef(), Value: structpb.NewNumberValue(*stats.WinRate)}}
	}
	return connect.NewResponse(&v1.GetExperimentPerformanceResponse{
		Performance: &v1.KpiReport{Results: results, Statistics: stats},
	}), nil
}

func sidesFromRows(rows []experimentdb.ExperimentOutcomeStat) (gain, loss *v1.OutcomeSide) {
	for _, row := range rows {
		dist := &v1.OutcomeDist{
			Average: float64(row.Average),
			P10:     float64(row.P10),
			P25:     float64(row.P25),
			P50:     float64(row.P50),
			P75:     float64(row.P75),
			P90:     float64(row.P90),
			P95:     float64(row.P95),
		}
		var side *v1.OutcomeSide
		if row.Side == performance.SideGain {
			if gain == nil {
				gain = &v1.OutcomeSide{}
			}
			side = gain
		} else {
			if loss == nil {
				loss = &v1.OutcomeSide{}
			}
			side = loss
		}
		if row.Unit == performance.UnitPercent {
			side.Percentage = dist
		} else {
			side.Absolute = dist
		}
	}
	return gain, loss
}
