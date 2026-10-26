package job

import (
	"errors"

	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/bars"
	indicatorrun "github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/indicators/run"
	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/lake"
)

// BarRange is the aggregated OHLCV series of a market for an experiment window.
type BarRange struct {
	// Bars start up to one indicator window before TradeFromMs (warmup) and end at the last
	// closed bar below `to`; incomplete aggregation windows are dropped.
	Bars []bars.CandleAggregation
	// EvalFromMs is the first bar BuildRangeRows keeps (`from` rounded up to a bar that exists).
	EvalFromMs int64
	// TradeFromMs is the first bar that can trade: the bar after EvalFromMs. The Go experiment
	// path evaluates its first bar without a previous bar (the KPI window starts there), so it
	// cannot act on a cross at that bar; the sweep uses it only to warm up.
	TradeFromMs int64
}

// BuildBarRange reads the lake and returns the bars BuildRangeRows evaluates for [from, to),
// preceded by the warmup bars its indicators read (Window bars before each kept bar).
func BuildBarRange(dataDir, symbol string, from, to, nowMS int64) (BarRange, error) {
	if !validPipelineSymbol(symbol) {
		return BarRange{}, errors.New("invalid symbol")
	}
	raw, err := lake.Read(rawParquetPaths(dataDir, symbol)...)
	if err != nil {
		return BarRange{}, err
	}
	spec := AggregationSpec(raw.IntervalMS)
	if rem := from % spec.DurationMS; rem != 0 {
		from += spec.DurationMS - rem
	}
	ordered := bars.Ordered(spec, raw.Candles)
	first, last := -1, -1
	for i, agg := range ordered {
		if agg.OpenTime < from || agg.OpenTime+spec.DurationMS > to || agg.OpenTime+spec.DurationMS > nowMS {
			continue
		}
		if first < 0 {
			first = i
		}
		last = i
	}
	if first < 0 {
		return BarRange{}, lake.ErrMissing
	}
	eval := ordered[first].OpenTime
	first = max(0, first-(indicatorrun.Window(spec.DurationMS)-1))
	return BarRange{Bars: ordered[first : last+1], EvalFromMs: eval, TradeFromMs: eval + spec.DurationMS}, nil
}
