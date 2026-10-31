package backtest

import (
	"strings"
	"time"

	"github.com/MaxBernstetter/dagobert/src/calculation-performance"
	signalrunner "github.com/MaxBernstetter/dagobert/src/calculation-signal/runner"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/pgcatalog"
)

// periodStatRows projects a report onto the experiment_instance_period_stat rows.
func periodStatRows(report signalrunner.InstanceReport, fills []signalrunner.StrategyKPIFill) []pgcatalog.PeriodStatRow {
	return periodStatsFromReport(report.Statistics, fills, report.Performance)
}

func periodStatsFromReport(stats signalrunner.RunStatistics, fills []signalrunner.StrategyKPIFill, perf performance.Result) []pgcatalog.PeriodStatRow {
	monthTrades := monthlySellCounts(fills)
	monthPerf := periodIndex(perf.Monthly)
	yearPerf := periodIndex(perf.Yearly)
	var out []pgcatalog.PeriodStatRow
	seenMonth := map[string]bool{}
	for _, p := range stats.MonthlyReturns {
		seenMonth[p.Period] = true
		ret := p.Return
		row := pgcatalog.PeriodStatRow{
			Kind:      "month",
			Key:       p.Period,
			ReturnPct: &ret,
		}
		if n, ok := monthTrades[p.Period]; ok {
			v := int32(n)
			row.TradeCount = &v
		}
		if bucket, ok := monthPerf[p.Period]; ok {
			trades := int32(bucket.Trades)
			wins := int32(bucket.Wins)
			row.TradeCount = &trades
			row.WinCount = &wins
		}
		out = append(out, row)
	}
	for _, bucket := range perf.Monthly {
		if seenMonth[bucket.Period] {
			continue
		}
		trades := int32(bucket.Trades)
		wins := int32(bucket.Wins)
		out = append(out, pgcatalog.PeriodStatRow{
			Kind:       "month",
			Key:        bucket.Period,
			TradeCount: &trades,
			WinCount:   &wins,
		})
	}
	seenYear := map[string]bool{}
	for _, p := range stats.YearlyReturns {
		seenYear[p.Period] = true
		ret := p.Return
		row := pgcatalog.PeriodStatRow{
			Kind:      "year",
			Key:       p.Period,
			ReturnPct: &ret,
		}
		if bucket, ok := yearPerf[p.Period]; ok {
			trades := int32(bucket.Trades)
			wins := int32(bucket.Wins)
			row.TradeCount = &trades
			row.WinCount = &wins
		}
		out = append(out, row)
	}
	for _, bucket := range perf.Yearly {
		if seenYear[bucket.Period] {
			continue
		}
		trades := int32(bucket.Trades)
		wins := int32(bucket.Wins)
		out = append(out, pgcatalog.PeriodStatRow{
			Kind:       "year",
			Key:        bucket.Period,
			TradeCount: &trades,
			WinCount:   &wins,
		})
	}
	return out
}

func periodIndex(rows []performance.PeriodCount) map[string]performance.PeriodCount {
	out := make(map[string]performance.PeriodCount, len(rows))
	for _, row := range rows {
		out[row.Period] = row
	}
	return out
}

func monthlySellCounts(fills []signalrunner.StrategyKPIFill) map[string]int {
	out := map[string]int{}
	for _, f := range fills {
		if strings.ToLower(f.Side) != "sell" {
			continue
		}
		key := time.UnixMilli(f.CandleCloseMs).UTC().Format("2006-01")
		out[key]++
	}
	return out
}
