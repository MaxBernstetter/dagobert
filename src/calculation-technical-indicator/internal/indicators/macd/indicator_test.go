package macd

import (
	"testing"

	"github.com/MaxBernstetter/dagobert/src/calculation-technical-indicator/internal/bars"
)

func TestMacdHistogramUptrendPositive(t *testing.T) {
	spec := bars.AggregationSpec{SourceIntervalMS: 3_600_000, DurationMS: 10_800_000}
	series := make([]bars.CandleAggregation, 0, 12)
	price := 100.0
	for i := 0; i < 12; i++ {
		price += 2
		series = append(series, bars.CandleAggregation{
			Open: price, High: price + 1, Low: price - 1, Close: price,
			OpenTime: int64(i) * spec.DurationMS, DurationMS: spec.DurationMS,
		})
	}
	v, ok := Indicator{}.Calculate(series, 0)
	if !ok {
		t.Fatal("expected macd")
	}
	if v < -1e-6 {
		t.Fatalf("expected non-negative histogram on uptrend, got %v", v)
	}
}
