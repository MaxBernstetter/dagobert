package job

import (
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/bars"
	indicatorrun "github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/indicators/run"
	"github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/lake"
	"github.com/MaxBernstetter/trading-copilot/src/calculation-technical-indicator/internal/store"
)

// BuildRangeRows computes KPI bodies for closed bars in [from, to) without writing SQLite.
func BuildRangeRows(dataDir, symbol string, from, to, nowMS int64) ([]store.Row, error) {
	if _, ok := symbols[symbol]; !ok {
		return nil, errors.New("invalid symbol")
	}
	raw, err := lake.Read(filepath.Join(dataDir, "parquet", symbol+".parquet"))
	if err != nil {
		return nil, err
	}
	spec := AggregationSpec(raw.IntervalMS)
	if rem := from % spec.DurationMS; rem != 0 {
		from += spec.DurationMS - rem
	}
	ordered := bars.Ordered(spec, raw.Candles)
	window := indicatorrun.Window(spec.DurationMS)
	rows := make([]store.Row, 0, len(ordered))
	for i, agg := range ordered {
		if agg.OpenTime < from || agg.OpenTime+spec.DurationMS > to || agg.OpenTime+spec.DurationMS > nowMS {
			continue
		}
		start := i - window + 1
		if start < 0 {
			start = 0
		}
		results, err := indicatorrun.Results(ordered[start : i+1])
		if err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(kpiBody{
			Open:      agg.Open,
			High:      agg.High,
			Low:       agg.Low,
			Close:     agg.Close,
			Volume:    agg.Volume,
			OpenTime:  agg.OpenTime,
			CloseTime: agg.CloseTime,
			Results:   results,
		})
		if err != nil {
			return nil, err
		}
		rows = append(rows, store.Row{
			Symbol:     symbol,
			BarOpen:    agg.OpenTime,
			Body:       string(encoded),
			Source:     "backfill",
			DataSource: raw.DataSource,
		})
	}
	if len(rows) == 0 {
		return nil, lake.ErrMissing
	}
	return rows, nil
}
