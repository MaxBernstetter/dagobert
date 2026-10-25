package framework

import (
	"math"
	"testing"
)

func TestComputeCAGR(t *testing.T) {
	v, ok := computeCAGR(100, 121, 1)
	if !ok || math.Abs(v-0.21) > 0.001 {
		t.Fatalf("got %v ok=%v", v, ok)
	}
}

func TestComputeSharpePositive(t *testing.T) {
	curve := []equityPoint{}
	v := 100.0
	for i := 0; i < 50; i++ {
		curve = append(curve, equityPoint{CloseMs: int64(i) * strategyKPIBarMS, Value: v})
		v *= 1.001 + float64(i%5)*0.0001
	}
	s, ok := computeSharpe(curve)
	if !ok || s <= 0 {
		t.Fatalf("sharpe %v ok=%v", s, ok)
	}
}

func TestBuildInstanceReportUsesGivenStartCash(t *testing.T) {
	fills := []StrategyKPIFill{
		{CandleCloseMs: 3000, Side: "buy", BaseQty: 1, QuoteQty: 100_000, Price: 100_000},
	}
	bars := []StrategyKPIBar{
		{CloseMs: 1000, Close: 100_000},
		{CloseMs: 3000, Close: 100_000},
	}
	missing := BuildInstanceReportFromSim(RunReport{}, "pb", "BTCUSDT", 1000, 3000, 0, fills, bars)
	if missing.KPIs.Comparison != nil {
		t.Fatalf("zero start cash invented a book: %+v", missing.KPIs.Comparison)
	}
	got := BuildInstanceReportFromSim(RunReport{}, "pb", "BTCUSDT", 1000, 3000, 100_000, fills, bars)
	if got.KPIs.Comparison == nil || got.KPIs.Comparison.StartCash != 100_000 {
		t.Fatalf("comparison = %+v", got.KPIs.Comparison)
	}
}

func TestReplayAndActiveReturn(t *testing.T) {
	bars := []StrategyKPIBar{
		{CloseMs: 3000, Close: 100},
		{CloseMs: 6000, Close: 110},
		{CloseMs: 9000, Close: 105},
	}
	fills := []StrategyKPIFill{
		{CandleCloseMs: 3000, Side: "buy", BaseQty: 0.5, QuoteQty: 50, Price: 100},
	}
	p, b := ReplayEquityCurves(fills, bars, 100)
	if len(p) != 3 || len(b) != 3 {
		t.Fatalf("len p=%d b=%d", len(p), len(b))
	}
	results := ComputeStrategyKPIValues(p, b, 3000, 9000)
	if len(results) == 0 {
		t.Fatal("no results")
	}
}
