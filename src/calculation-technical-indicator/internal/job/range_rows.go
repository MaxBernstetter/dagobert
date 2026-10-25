package job

import (
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/bars"
	indicatorrun "github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/indicators/run"
	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/lake"
	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/store"
	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/tiparquet"
	"github.com/MaxBernstetter/dagobert/src/shared"
)

// BuildRangeRows computes KPI bodies for closed bars in [from, to) without writing SQLite.
// Warm TI parquet cache is used when year partitions cover the range.
func BuildRangeRows(dataDir, symbol string, from, to, nowMS int64) ([]store.Row, error) {
	if !validPipelineSymbol(symbol) {
		return nil, errors.New("invalid symbol")
	}
	provider, base, quote := tiMarket(symbol)
	algo := "v1"
	// Probe aggregation duration from a cheap candle read only on cold path;
	// warm path assumes experiment default 3h (10_800_000) — same as Experiment Suite.
	const defaultAggMS int64 = 10_800_000
	if cached, ok, err := tiparquet.LoadRange(dataDir, provider, base, quote, algo, from, to, defaultAggMS); err != nil {
		return nil, err
	} else if ok {
		out := make([]store.Row, 0, len(cached))
		for _, r := range cached {
			if r.BarOpen+defaultAggMS > nowMS {
				continue
			}
			body, err := encodeCachedBody(r)
			if err != nil {
				return nil, err
			}
			ds, ok := shared.ParseDataSource(provider)
			if !ok {
				ds = shared.DefaultDataSource()
			}
			out = append(out, store.Row{
				Symbol:     symbol,
				BarOpen:    r.BarOpen,
				Body:       body,
				Source:     "ti-cache",
				DataSource: ds,
			})
		}
		if len(out) > 0 {
			return out, nil
		}
	}

	raw, err := lake.Read(rawParquetPaths(dataDir, symbol)...)
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
	tiRows := make([]tiparquet.Row, 0, len(ordered))
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
		resultsJSON, _ := json.Marshal(results)
		rows = append(rows, store.Row{
			Symbol:     symbol,
			BarOpen:    agg.OpenTime,
			Body:       string(encoded),
			Source:     "backfill",
			DataSource: raw.DataSource,
		})
		tiRows = append(tiRows, tiparquet.Row{
			BarOpen:     agg.OpenTime,
			Open:        agg.Open,
			High:        agg.High,
			Low:         agg.Low,
			Close:       agg.Close,
			Volume:      agg.Volume,
			CloseTime:   agg.CloseTime,
			ResultsJSON: string(resultsJSON),
		})
	}
	if len(rows) == 0 {
		return nil, lake.ErrMissing
	}
	_ = tiparquet.WriteRange(dataDir, provider, base, quote, algo, tiRows)
	return rows, nil
}

func tiMarket(symbol string) (provider, base, quote string) {
	provider = strings.TrimSpace(os.Getenv("INGEST_PROVIDER"))
	if provider == "" {
		provider = shared.DefaultDataSource().String()
	}
	base, quote, _ = shared.SplitPipelineSymbol(symbol)
	return provider, base, quote
}

func encodeCachedBody(r tiparquet.Row) (string, error) {
	var results json.RawMessage
	if r.ResultsJSON != "" {
		results = json.RawMessage(r.ResultsJSON)
	} else {
		results = json.RawMessage("[]")
	}
	// Re-wrap as kpiBody-compatible JSON for Window loaders.
	type body struct {
		Open      float64         `json:"open"`
		High      float64         `json:"high"`
		Low       float64         `json:"low"`
		Close     float64         `json:"close"`
		Volume    float64         `json:"volume"`
		OpenTime  int64           `json:"open_time"`
		CloseTime int64           `json:"close_time"`
		Results   json.RawMessage `json:"results"`
	}
	encoded, err := json.Marshal(body{
		Open: r.Open, High: r.High, Low: r.Low, Close: r.Close, Volume: r.Volume,
		OpenTime: r.BarOpen, CloseTime: r.CloseTime, Results: results,
	})
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
