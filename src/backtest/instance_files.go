package backtest

import (
	"github.com/MaxBernstetter/dagobert/src/calculation-performance"
	signalrunner "github.com/MaxBernstetter/dagobert/src/calculation-signal/runner"
	"github.com/MaxBernstetter/dagobert/src/internal/instanceparquet"
)

// writeInstanceFiles writes the Parquet files of one instance next to report.json.
func writeInstanceFiles(experimentID string, r instanceResult) error {
	meta := instanceparquet.Meta{
		ExperimentID: experimentID,
		InstanceID:   r.inst.ID,
		PlaybookKey:  r.inst.PlaybookKey,
		Symbol:       r.sym,
	}
	if err := instanceparquet.WriteOrders(r.ws, meta, r.orders); err != nil {
		return err
	}
	if err := instanceparquet.WriteJournals(r.ws, meta, r.journals); err != nil {
		return err
	}
	if err := instanceparquet.WriteFills(r.ws, meta, fillRowsFromKPI(r.fills)); err != nil {
		return err
	}
	return instanceparquet.WriteTradeOutcomes(r.ws, meta, tradeOutcomeParquetRows(r.report.Performance))
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
