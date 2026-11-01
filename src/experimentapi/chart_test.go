package experimentapi

import (
	"math"
	"testing"
)

func TestBuildChartBarsWindowAndIndicators(t *testing.T) {
	candles := []chartCandle{
		{OpenTime: 1_000, Open: 1, High: 2, Low: 0.5, Close: 1.5},
		{OpenTime: 2_000, Open: 3, High: 4, Low: 2, Close: 3.5},
		{OpenTime: 5_000, Open: 9, High: 10, Low: 8, Close: 9.5},
	}
	ti := map[int64]map[string]float64{
		2_000: {"sma-1h": 3.25},
	}
	meta := []chartIndicatorMeta{{ID: "sma-1h", Label: "SMA 1h", Unit: "price", Pane: "price", Style: "line"}}

	got := buildMarketSeries(candles, ti, meta, 2, 4)
	if len(got.TimeSec) != 1 {
		t.Fatalf("expected 1 bar in window, got %d", len(got.TimeSec))
	}
	if got.TimeSec[0] != 2 || got.Open[0] != 3 || got.High[0] != 4 || got.Low[0] != 2 || got.Close[0] != 3.5 {
		t.Fatalf("mapped bar = %+v", got)
	}
	if len(got.Indicators) != 1 || got.Indicators[0].Value[0] != 3.25 {
		t.Fatalf("indicators = %+v", got.Indicators)
	}
}

func TestBuildChartBarsSortsByTime(t *testing.T) {
	candles := []chartCandle{
		{OpenTime: 3_000, Close: 3},
		{OpenTime: 1_000, Close: 1},
	}
	meta := []chartIndicatorMeta{{ID: "sma-1h"}}
	got := buildMarketSeries(candles, nil, meta, 0, 10)
	if len(got.TimeSec) != 2 || got.TimeSec[0] != 1 || got.TimeSec[1] != 3 {
		t.Fatalf("order = %+v", got.TimeSec)
	}
	if len(got.Indicators) != 1 || !math.IsNaN(got.Indicators[0].Value[0]) {
		t.Fatalf("missing TI sample should be NaN, got %+v", got.Indicators)
	}
}

func TestEquityFromFillsKeepsFullStartingBook(t *testing.T) {
	fills := []chartFill{{closeMs: 3000, side: "buy", base: 1, quote: 100000, price: 100000}}
	points := equityFromFills(fills, 100_000)
	if len(points) != 2 {
		t.Fatalf("points = %d", len(points))
	}
	if points[0].Value != 100_000 || points[1].Value != 100_000 {
		t.Fatalf("equity = %v then %v", points[0].Value, points[1].Value)
	}
}

func TestNumericIndicatorsAndID(t *testing.T) {
	meta := map[string]chartIndicatorMeta{}
	raw := `[{"def":{"method":"sma","time_frame_hr":4},"value":12.5,"unit":"price"},{"def":"{\"type\":\"Rsi\"}","value":55,"unit":"index"},{"def":{"type":"Macd","method":"histogram"},"value":-3,"unit":"price"}]`
	got := numericIndicators(raw, meta)
	if got["sma-4h"] != 12.5 || got["Rsi"] != 55 || got["Macd"] != -3 {
		t.Fatalf("values = %+v", got)
	}
	if meta["sma-4h"].Label != "SMA 4h" || meta["Rsi"].Label != "Rsi" {
		t.Fatalf("meta = %+v", meta)
	}
	if meta["Rsi"].Unit != "index" || meta["Rsi"].Style != "line" || len(meta["Rsi"].Reference) != 2 {
		t.Fatalf("rsi meta = %+v", meta["Rsi"])
	}
	if meta["Macd"].Style != "histogram" || meta["Macd"].Unit != "price" {
		t.Fatalf("macd meta = %+v", meta["Macd"])
	}
	if id, label := indicatorID("", "", 0); id != "" || label != "" {
		t.Fatalf("empty id = %q %q", id, label)
	}
}
