package backtest

import (
	"github.com/MaxBernstetter/dagobert/src/calculation-performance"
	signalrunner "github.com/MaxBernstetter/dagobert/src/calculation-signal/runner"
	"github.com/MaxBernstetter/dagobert/src/internal/instanceparquet"
)

// instanceFiles is everything one instance contributes to its shard.
func instanceFiles(r instanceResult, reportJSON []byte) instanceparquet.InstanceData {
	return instanceparquet.InstanceData{
		InstanceID: r.inst.ID,
		ReportJSON: reportJSON,
		Fills:      fillRowsFromKPI(r.fills),
		Orders:     r.orders,
		Journals:   r.journals,
		Outcomes:   tradeOutcomeParquetRows(r.report.Performance),
		Periods:    periodStatRows(r.report, r.fills),
	}
}

func fillRowsFromKPI(fills []signalrunner.StrategyKPIFill) []instanceparquet.Fill {
	out := make([]instanceparquet.Fill, 0, len(fills))
	for _, f := range fills {
		out = append(out, instanceparquet.Fill{TsMs: f.CandleCloseMs, Side: f.Side, BaseQty: f.BaseQty, QuoteQty: f.QuoteQty, Price: f.Price})
	}
	return out
}

// tradeOutcomeParquetRows is outcomeRows in the Parquet layout.
func tradeOutcomeParquetRows(perf performance.Result) []instanceparquet.TradeOutcome {
	out := make([]instanceparquet.TradeOutcome, 0, len(perf.Trades))
	for i, tr := range perf.Trades {
		row := instanceparquet.TradeOutcome{Ordinal: int32(i), Side: int32(tr.Side), Quote: float32(tr.Quote)}
		if tr.Percent != nil {
			p := float32(*tr.Percent)
			row.Percent = &p
		}
		out = append(out, row)
	}
	return out
}
