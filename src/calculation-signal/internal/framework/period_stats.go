package framework

import (
	"math"
	"sort"
	"time"
)

// AvgMed is a nested average/median pair for report.json statistics.
type AvgMed struct {
	Average float64 `json:"average"`
	Median  float64 `json:"median"`
}

// PeriodReturn is one calendar bucket return.
type PeriodReturn struct {
	Period string  `json:"period"`
	Return float64 `json:"return"`
}

// RunStatistics is period performance for an instance report.
type RunStatistics struct {
	DurationMonths float64        `json:"durationMonths"`
	DurationYears  float64        `json:"durationYears"`
	MonthlyReturn  *AvgMed        `json:"monthlyReturn,omitempty"`
	YearlyReturn   *AvgMed        `json:"yearlyReturn,omitempty"`
	TradesPerMonth *AvgMed        `json:"tradesPerMonth,omitempty"`
	MonthlyReturns []PeriodReturn `json:"monthlyReturns,omitempty"`
	YearlyReturns  []PeriodReturn `json:"yearlyReturns,omitempty"`
}

// StrategyKPIComparison mirrors the UI comparison block.
type StrategyKPIComparison struct {
	StartCash       float64 `json:"startCash"`
	StrategyReturn  float64 `json:"strategyReturn"`
	BuyHoldReturn   float64 `json:"buyHoldReturn"`
}

func avgMed(values []float64) *AvgMed {
	nums := make([]float64, 0, len(values))
	for _, v := range values {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			nums = append(nums, v)
		}
	}
	if len(nums) == 0 {
		return nil
	}
	sum := 0.0
	for _, v := range nums {
		sum += v
	}
	sorted := append([]float64(nil), nums...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	med := sorted[mid]
	if len(sorted)%2 == 0 {
		med = (sorted[mid-1] + sorted[mid]) / 2
	}
	return &AvgMed{Average: sum / float64(len(nums)), Median: med}
}

func utcMonthKey(ms int64) string {
	t := time.UnixMilli(ms).UTC()
	return t.Format("2006-01")
}

func utcYearKey(ms int64) string {
	t := time.UnixMilli(ms).UTC()
	return t.Format("2006")
}

func periodReturnsFromCurve(curve []equityPoint, bucketKey func(int64) string) []PeriodReturn {
	if len(curve) < 2 {
		return nil
	}
	type bucket struct {
		first, last equityPoint
	}
	order := make([]string, 0)
	buckets := map[string]*bucket{}
	for _, p := range curve {
		key := bucketKey(p.CloseMs)
		b, ok := buckets[key]
		if !ok {
			buckets[key] = &bucket{first: p, last: p}
			order = append(order, key)
			continue
		}
		if p.CloseMs < b.first.CloseMs {
			b.first = p
		}
		if p.CloseMs >= b.last.CloseMs {
			b.last = p
		}
	}
	out := make([]PeriodReturn, 0, len(order))
	for _, key := range order {
		b := buckets[key]
		if b.first.Value > 0 && b.last.Value > 0 {
			out = append(out, PeriodReturn{Period: key, Return: b.last.Value/b.first.Value - 1})
		}
	}
	return out
}

func monthlyTradeCounts(fills []StrategyKPIFill) []float64 {
	buckets := map[string]float64{}
	for _, f := range fills {
		if f.Side != "sell" {
			continue
		}
		key := utcMonthKey(f.CandleCloseMs)
		buckets[key]++
	}
	out := make([]float64, 0, len(buckets))
	for _, n := range buckets {
		out = append(out, n)
	}
	return out
}

// ComputeRunStatistics builds nested avg/median stats plus period return series.
func ComputeRunStatistics(portfolio []equityPoint, fills []StrategyKPIFill, fromMs, toMs int64) RunStatistics {
	years := float64(toMs-fromMs) / (365.25 * 24 * float64(time.Hour/time.Millisecond))
	if years < 0 {
		years = 0
	}
	monthly := periodReturnsFromCurve(portfolio, utcMonthKey)
	yearly := periodReturnsFromCurve(portfolio, utcYearKey)
	monthlyVals := make([]float64, len(monthly))
	for i, p := range monthly {
		monthlyVals[i] = p.Return
	}
	yearlyVals := make([]float64, len(yearly))
	for i, p := range yearly {
		yearlyVals[i] = p.Return
	}
	return RunStatistics{
		DurationMonths: years * 12,
		DurationYears:  years,
		MonthlyReturn:  avgMed(monthlyVals),
		YearlyReturn:   avgMed(yearlyVals),
		TradesPerMonth: avgMed(monthlyTradeCounts(fills)),
		MonthlyReturns: monthly,
		YearlyReturns:  yearly,
	}
}

// BuildComparison returns strategy vs buy-hold returns from equity curves.
func BuildComparison(portfolio, benchmark []equityPoint, startCash float64) *StrategyKPIComparison {
	if len(portfolio) < 2 || startCash <= 0 {
		return nil
	}
	startV := portfolio[0].Value
	endV := portfolio[len(portfolio)-1].Value
	if startV <= 0 {
		return nil
	}
	comp := &StrategyKPIComparison{
		StartCash:      startCash,
		StrategyReturn: endV/startV - 1,
	}
	if len(benchmark) >= 2 && benchmark[0].Value > 0 {
		comp.BuyHoldReturn = benchmark[len(benchmark)-1].Value/benchmark[0].Value - 1
	}
	return comp
}
