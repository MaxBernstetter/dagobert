package experimentapi

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"

	"github.com/MaxBernstetter/dagobert/src/calculation-performance"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/pgcatalog"
	"github.com/jackc/pgx/v5"
)

type outcomeDist struct {
	Average float64 `json:"average"`
	P10     float64 `json:"p10"`
	P25     float64 `json:"p25"`
	P50     float64 `json:"p50"`
	P75     float64 `json:"p75"`
	P90     float64 `json:"p90"`
	P95     float64 `json:"p95"`
}

type outcomeSide struct {
	Absolute   *outcomeDist `json:"absolute,omitempty"`
	Percentage *outcomeDist `json:"percentage,omitempty"`
}

func winRateDef() kpisDef {
	return kpisDef{
		Slug:   "win-rate",
		Title:  "Win rate",
		Label:  "Win rate",
		Kind:   "strategy-kpi",
		Unit:   "ratio",
		Format: "percent",
	}
}

func (s *Server) attachInstancePerformance(ctx context.Context, inst experimentdb.ExperimentInstance, resp *kpisResponse) error {
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
		if row.Def.Slug == "win-rate" {
			continue
		}
		kept = append(kept, row)
	}
	resp.Results = kept
	if result.WinRate != nil {
		raw, err := json.Marshal(*result.WinRate)
		if err != nil {
			return err
		}
		resp.Results = append(resp.Results, kpisResult{Def: winRateDef(), Value: raw})
	}
	applyPerformanceStats(&resp.Statistics, result)
	return nil
}

func applyPerformanceStats(stats *kpisStatistics, result performance.Result) {
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

func sideFrom(side *performance.SideStats) *outcomeSide {
	if side == nil {
		return nil
	}
	out := &outcomeSide{Absolute: distFrom(side.Absolute), Percentage: distFrom(side.Percentage)}
	if out.Absolute == nil && out.Percentage == nil {
		return nil
	}
	return out
}

func distFrom(d *performance.Distribution) *outcomeDist {
	if d == nil {
		return nil
	}
	return &outcomeDist{
		Average: d.Average,
		P10:     d.P10,
		P25:     d.P25,
		P50:     d.P50,
		P75:     d.P75,
		P90:     d.P90,
		P95:     d.P95,
	}
}

// handleExperimentPerformance serves the stored experiment reduction.
func (s *Server) handleExperimentPerformance(w http.ResponseWriter, r *http.Request) {
	id, err := parseExperimentID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	row, err := s.q.GetExperimentPerformance(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "performance not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	periods, err := s.q.ListExperimentPeriodStats(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	outcomes, err := s.q.ListExperimentOutcomeStats(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
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
	stats := kpisStatistics{}
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
	var results []kpisResult
	if stats.WinRate != nil {
		raw, err := json.Marshal(*stats.WinRate)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		results = []kpisResult{{Def: winRateDef(), Value: raw}}
	}
	writeJSON(w, http.StatusOK, kpisResponse{
		Results:    results,
		Statistics: stats,
	})
}

func sidesFromRows(rows []experimentdb.ExperimentOutcomeStat) (gain, loss *outcomeSide) {
	for _, row := range rows {
		dist := &outcomeDist{
			Average: float64(row.Average),
			P10:     float64(row.P10),
			P25:     float64(row.P25),
			P50:     float64(row.P50),
			P75:     float64(row.P75),
			P90:     float64(row.P90),
			P95:     float64(row.P95),
		}
		var side *outcomeSide
		if row.Side == performance.SideGain {
			if gain == nil {
				gain = &outcomeSide{}
			}
			side = gain
		} else {
			if loss == nil {
				loss = &outcomeSide{}
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
