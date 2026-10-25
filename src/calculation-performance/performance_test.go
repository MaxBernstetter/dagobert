package performance

import (
	"math"
	"testing"
	"time"
)

func ms(y int, m time.Month, d int) int64 {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).UnixMilli()
}

func TestEvaluateRoundTrips(t *testing.T) {
	fills := []Fill{
		{CandleCloseMs: ms(2024, 1, 2), Side: "buy", BaseQty: 1, QuoteQty: 100},
		{CandleCloseMs: ms(2024, 1, 3), Side: "buy", BaseQty: 1, QuoteQty: 200},
		{CandleCloseMs: ms(2024, 1, 10), Side: "sell", BaseQty: 2, QuoteQty: 360},
		{CandleCloseMs: ms(2024, 2, 1), Side: "buy", BaseQty: 1, QuoteQty: 100},
		{CandleCloseMs: ms(2024, 2, 2), Side: "sell", BaseQty: 1, QuoteQty: 100},
		{CandleCloseMs: ms(2024, 3, 1), Side: "buy", BaseQty: 1, QuoteQty: 100},
		{CandleCloseMs: ms(2024, 3, 2), Side: "sell", BaseQty: 1, QuoteQty: 80},
		{CandleCloseMs: ms(2024, 4, 1), Side: "sell", BaseQty: 1, QuoteQty: 50},
	}
	got := Evaluate(fills, 0, 0)
	if got.Wins != 1 || got.Losses != 1 || got.Breakeven != 1 {
		t.Fatalf("counts win=%d loss=%d even=%d", got.Wins, got.Losses, got.Breakeven)
	}
	if got.WinRate == nil || math.Abs(*got.WinRate-1.0/3.0) > 1e-9 {
		t.Fatalf("win rate %v", got.WinRate)
	}
	if got.Gain == nil || got.Gain.Absolute == nil || math.Abs(got.Gain.Absolute.Average-60) > 1e-9 {
		t.Fatalf("gain %+v", got.Gain)
	}
	if got.Gain.Percentage == nil || math.Abs(got.Gain.Percentage.Average-0.2) > 1e-9 {
		t.Fatalf("gain pct %+v", got.Gain.Percentage)
	}
	if got.Loss == nil || got.Loss.Absolute == nil || math.Abs(got.Loss.Absolute.Average-20) > 1e-9 {
		t.Fatalf("loss %+v", got.Loss)
	}
	if len(got.Monthly) != 3 {
		t.Fatalf("months %+v", got.Monthly)
	}
	for _, p := range got.Monthly {
		if p.Period == "2024-04" {
			t.Fatal("empty inventory sell counted")
		}
	}
}

func TestOneTradeLadder(t *testing.T) {
	fills := []Fill{
		{CandleCloseMs: ms(2024, 1, 1), Side: "buy", BaseQty: 1, QuoteQty: 50},
		{CandleCloseMs: ms(2024, 1, 2), Side: "sell", BaseQty: 1, QuoteQty: 80},
	}
	got := Evaluate(fills, 0, 0)
	d := got.Gain.Absolute
	if d.P10 != 30 || d.P50 != 30 || d.P90 != 30 || d.P95 != 30 {
		t.Fatalf("ladder %+v", d)
	}
}

func TestP90IsUpperTail(t *testing.T) {
	var values []float64
	for i := 1; i <= 10; i++ {
		values = append(values, float64(i))
	}
	d := distribution(values)
	if math.Abs(d.P90-9.1) > 1e-9 {
		t.Fatalf("p90 %v", d.P90)
	}
	above := 0
	for _, v := range d.Samples {
		if v > d.P90 {
			above++
		}
	}
	if above != 1 {
		t.Fatalf("samples above p90: %d", above)
	}
}

func TestMonthWithoutSellsOmitted(t *testing.T) {
	fills := []Fill{
		{CandleCloseMs: ms(2024, 1, 1), Side: "buy", BaseQty: 1, QuoteQty: 10},
		{CandleCloseMs: ms(2024, 1, 20), Side: "sell", BaseQty: 1, QuoteQty: 12},
		{CandleCloseMs: ms(2024, 2, 1), Side: "buy", BaseQty: 1, QuoteQty: 10},
	}
	got := Evaluate(fills, ms(2024, 1, 1), ms(2024, 3, 1))
	if len(got.Monthly) != 1 || got.Monthly[0].Period != "2024-01" {
		t.Fatalf("monthly %+v", got.Monthly)
	}
	if got.MonthlyWinRate == nil || got.MonthlyWinRate.Average != 1 {
		t.Fatalf("avg %+v", got.MonthlyWinRate)
	}
}

func TestAggregateTradeWeighted(t *testing.T) {
	win := Evaluate([]Fill{
		{CandleCloseMs: ms(2024, 1, 1), Side: "buy", BaseQty: 1, QuoteQty: 10},
		{CandleCloseMs: ms(2024, 1, 2), Side: "sell", BaseQty: 1, QuoteQty: 12},
	}, 0, 0)
	var losses []Fill
	for i := 0; i < 9; i++ {
		day := 10 + i
		losses = append(losses,
			Fill{CandleCloseMs: ms(2024, 6, day), Side: "buy", BaseQty: 1, QuoteQty: 10},
			Fill{CandleCloseMs: ms(2024, 6, day), Side: "sell", BaseQty: 1, QuoteQty: 9},
		)
	}
	lose := Evaluate(losses, 0, 0)
	empty := Evaluate(nil, 0, 0)
	got := Aggregate([]Result{win, lose, empty})
	if got.Wins != 1 || got.Losses != 9 {
		t.Fatalf("counts %+v", got)
	}
	if got.WinRate == nil || math.Abs(*got.WinRate-0.1) > 1e-9 {
		t.Fatalf("pooled rate %v", got.WinRate)
	}
	mean := (*win.WinRate + *lose.WinRate) / 2
	if math.Abs(*got.WinRate-mean) < 1e-9 {
		t.Fatal("pooled rate equals the unweighted mean")
	}
	if len(got.Gain.Absolute.Samples) != 1 || len(got.Loss.Absolute.Samples) != 9 {
		t.Fatalf("samples gain %d loss %d", len(got.Gain.Absolute.Samples), len(got.Loss.Absolute.Samples))
	}
	if len(got.Monthly) != 2 {
		t.Fatalf("months %+v", got.Monthly)
	}
}
