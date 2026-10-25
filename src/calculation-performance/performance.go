// Package performance scores closed trades for one instance and rolls those
// results up to an experiment. It does not read the lake or a database.
package performance

import (
	"math"
	"sort"
	"strings"
	"time"
)

const (
	SideGain    = 1
	SideLoss    = 0
	UnitQuote   = 1
	UnitPercent = 0
)

// Fill is one execution fill already produced by an instance run.
type Fill struct {
	CandleCloseMs int64
	Side          string
	BaseQty       float64
	QuoteQty      float64
}

// PeriodCount is wins and closed trades in one UTC calendar bucket.
type PeriodCount struct {
	Period string `json:"period"`
	Wins   int    `json:"wins"`
	Trades int    `json:"trades"`
}

// AvgMed is the mean and median of per-bucket win rates.
type AvgMed struct {
	Average float64 `json:"average"`
	Median  float64 `json:"median"`
}

// Distribution is one side of the gain or loss ladder for a single unit.
// Samples are sorted ascending. Empty when that side has no samples.
type Distribution struct {
	Average float64   `json:"average"`
	P10     float64   `json:"p10"`
	P25     float64   `json:"p25"`
	P50     float64   `json:"p50"`
	P75     float64   `json:"p75"`
	P90     float64   `json:"p90"`
	P95     float64   `json:"p95"`
	Samples []float64 `json:"samples,omitempty"`
}

// SideStats is quote notional and percent of cost for one side.
type SideStats struct {
	Absolute   *Distribution `json:"absolute,omitempty"`
	Percentage *Distribution `json:"percentage,omitempty"`
}

// ClosedTrade is one winning or losing sell. Quote is a positive magnitude.
// Percent is nil when the cost basis of the base sold was zero.
type ClosedTrade struct {
	Quote   float64  `json:"quote"`
	Percent *float64 `json:"percent,omitempty"`
	Side    int      `json:"side"`
}

// Result is the performance of one instance or of a whole experiment.
type Result struct {
	Wins           int           `json:"wins"`
	Losses         int           `json:"losses"`
	Breakeven      int           `json:"breakeven"`
	WinRate        *float64      `json:"winRate,omitempty"`
	Monthly        []PeriodCount `json:"monthly,omitempty"`
	Yearly         []PeriodCount `json:"yearly,omitempty"`
	MonthlyWinRate *AvgMed       `json:"monthlyWinRate,omitempty"`
	YearlyWinRate  *AvgMed       `json:"yearlyWinRate,omitempty"`
	Gain           *SideStats    `json:"gain,omitempty"`
	Loss           *SideStats    `json:"loss,omitempty"`
	// Trades is evaluation order, not sorted. Aggregate pools these.
	Trades []ClosedTrade `json:"trades,omitempty"`
}

// ClosedTrades is wins + losses + breakevens.
func (r Result) ClosedTrades() int {
	return r.Wins + r.Losses + r.Breakeven
}

// Evaluate scores fills whose candle close is inside [fromMs, toMs].
// A zero bound on either side means that bound is open.
func Evaluate(fills []Fill, fromMs, toMs int64) Result {
	ordered := append([]Fill(nil), fills...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].CandleCloseMs < ordered[j].CandleCloseMs
	})

	var (
		base, costBasis float64
		trades          []ClosedTrade
		monthWins       = map[string]int{}
		monthTrades     = map[string]int{}
		yearWins        = map[string]int{}
		yearTrades      = map[string]int{}
		monthOrder      []string
		yearOrder       []string
	)
	note := func(ms int64, win bool) {
		mk := utcMonth(ms)
		if _, ok := monthTrades[mk]; !ok {
			monthOrder = append(monthOrder, mk)
		}
		monthTrades[mk]++
		if win {
			monthWins[mk]++
		}
		yk := utcYear(ms)
		if _, ok := yearTrades[yk]; !ok {
			yearOrder = append(yearOrder, yk)
		}
		yearTrades[yk]++
		if win {
			yearWins[yk]++
		}
	}

	for _, f := range ordered {
		if !inWindow(f.CandleCloseMs, fromMs, toMs) {
			continue
		}
		switch normSide(f.Side) {
		case "buy":
			if f.BaseQty <= 0 || f.QuoteQty <= 0 {
				continue
			}
			costBasis += f.QuoteQty
			base += f.BaseQty
		case "sell":
			if base <= 0 || f.BaseQty <= 0 || costBasis <= 0 {
				continue
			}
			sold := f.BaseQty
			quote := f.QuoteQty
			if sold > base {
				quote = f.QuoteQty * (base / sold)
				sold = base
			}
			if sold <= 0 {
				continue
			}
			avg := costBasis / base
			cost := avg * sold
			realized := quote - cost
			costBasis -= cost
			base -= sold
			if costBasis < 0 || math.Abs(costBasis) < 1e-9 {
				costBasis = 0
			}
			if base < 0 || math.Abs(base) < 1e-12 {
				base = 0
				costBasis = 0
			}
			win := realized > 0
			note(f.CandleCloseMs, win)
			if realized == 0 {
				continue
			}
			mag := realized
			if mag < 0 {
				mag = -mag
			}
			tr := ClosedTrade{Quote: mag, Side: SideLoss}
			if win {
				tr.Side = SideGain
			}
			if cost > 0 {
				pct := mag / cost
				tr.Percent = &pct
			}
			trades = append(trades, tr)
		}
	}
	// Breakeven is a closed trade that was noted but not appended.
	// Recount from the period maps so wins/losses/breakeven stay consistent.
	return finish(trades, monthOrder, monthWins, monthTrades, yearOrder, yearWins, yearTrades)
}

func finish(
	trades []ClosedTrade,
	monthOrder []string, monthWins, monthTrades map[string]int,
	yearOrder []string, yearWins, yearTrades map[string]int,
) Result {
	r := Result{Trades: trades}
	var gainQ, gainP, lossQ, lossP []float64
	for _, tr := range trades {
		if tr.Side == SideGain {
			r.Wins++
			gainQ = append(gainQ, tr.Quote)
			if tr.Percent != nil {
				gainP = append(gainP, *tr.Percent)
			}
			continue
		}
		r.Losses++
		lossQ = append(lossQ, tr.Quote)
		if tr.Percent != nil {
			lossP = append(lossP, *tr.Percent)
		}
	}
	closed := 0
	for _, n := range monthTrades {
		closed += n
	}
	r.Breakeven = closed - r.Wins - r.Losses
	if r.Breakeven < 0 {
		r.Breakeven = 0
	}
	if closed > 0 {
		rate := float64(r.Wins) / float64(closed)
		r.WinRate = &rate
	}
	r.Monthly = periodCounts(monthOrder, monthWins, monthTrades)
	r.Yearly = periodCounts(yearOrder, yearWins, yearTrades)
	r.MonthlyWinRate = avgMedRates(r.Monthly)
	r.YearlyWinRate = avgMedRates(r.Yearly)
	r.Gain = sideStats(gainQ, gainP)
	r.Loss = sideStats(lossQ, lossP)
	return r
}

// Assemble rebuilds a Result from closed trades and calendar buckets.
// The instance API uses it to derive the ladder from sample rows.
func Assemble(trades []ClosedTrade, monthly, yearly []PeriodCount) Result {
	return finish(
		trades,
		periodOrder(monthly), periodWins(monthly), periodTrades(monthly),
		periodOrder(yearly), periodWins(yearly), periodTrades(yearly),
	)
}

func periodOrder(rows []PeriodCount) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Period)
	}
	return out
}

func periodWins(rows []PeriodCount) map[string]int {
	out := make(map[string]int, len(rows))
	for _, row := range rows {
		out[row.Period] = row.Wins
	}
	return out
}

func periodTrades(rows []PeriodCount) map[string]int {
	out := make(map[string]int, len(rows))
	for _, row := range rows {
		out[row.Period] = row.Trades
	}
	return out
}

// Aggregate pools instance results. Trade counts are summed. An instance with
// no closed trades is ignored. The returned trades are the concatenation, and
// the ladder is rebuilt from that pool.
func Aggregate(results []Result) Result {
	var (
		trades     []ClosedTrade
		monthOrder []string
		yearOrder  []string
		monthWins  = map[string]int{}
		monthN     = map[string]int{}
		yearWins   = map[string]int{}
		yearN      = map[string]int{}
	)
	addPeriods := func(order *[]string, wins, n map[string]int, rows []PeriodCount) {
		for _, p := range rows {
			if p.Trades <= 0 {
				continue
			}
			if _, ok := n[p.Period]; !ok {
				*order = append(*order, p.Period)
			}
			wins[p.Period] += p.Wins
			n[p.Period] += p.Trades
		}
	}
	for _, r := range results {
		if r.ClosedTrades() == 0 {
			continue
		}
		trades = append(trades, r.Trades...)
		addPeriods(&monthOrder, monthWins, monthN, r.Monthly)
		addPeriods(&yearOrder, yearWins, yearN, r.Yearly)
	}
	return finish(trades, monthOrder, monthWins, monthN, yearOrder, yearWins, yearN)
}

func sideStats(quote, percent []float64) *SideStats {
	abs := distribution(quote)
	pct := distribution(percent)
	if abs == nil && pct == nil {
		return nil
	}
	return &SideStats{Absolute: abs, Percentage: pct}
}

func distribution(values []float64) *Distribution {
	if len(values) == 0 {
		return nil
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	sum := 0.0
	for _, v := range sorted {
		sum += v
	}
	return &Distribution{
		Average: sum / float64(len(sorted)),
		P10:     percentile(sorted, 0.10),
		P25:     percentile(sorted, 0.25),
		P50:     percentile(sorted, 0.50),
		P75:     percentile(sorted, 0.75),
		P90:     percentile(sorted, 0.90),
		P95:     percentile(sorted, 0.95),
		Samples: sorted,
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	idx := p * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	w := idx - float64(lo)
	return sorted[lo]*(1-w) + sorted[hi]*w
}

func periodCounts(order []string, wins, trades map[string]int) []PeriodCount {
	out := make([]PeriodCount, 0, len(order))
	for _, key := range order {
		n := trades[key]
		if n <= 0 {
			continue
		}
		out = append(out, PeriodCount{Period: key, Wins: wins[key], Trades: n})
	}
	return out
}

// PeriodRates is the mean and median of per-bucket win rates.
func PeriodRates(rows []PeriodCount) *AvgMed {
	return avgMedRates(rows)
}

func avgMedRates(rows []PeriodCount) *AvgMed {
	vals := make([]float64, 0, len(rows))
	for _, row := range rows {
		if row.Trades <= 0 {
			continue
		}
		vals = append(vals, float64(row.Wins)/float64(row.Trades))
	}
	if len(vals) == 0 {
		return nil
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	sorted := append([]float64(nil), vals...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	med := sorted[mid]
	if len(sorted)%2 == 0 {
		med = (sorted[mid-1] + sorted[mid]) / 2
	}
	return &AvgMed{Average: sum / float64(len(vals)), Median: med}
}

func inWindow(ms, fromMs, toMs int64) bool {
	if fromMs > 0 && ms < fromMs {
		return false
	}
	if toMs > 0 && ms > toMs {
		return false
	}
	return true
}

func normSide(side string) string {
	return strings.ToLower(strings.TrimSpace(side))
}

func utcMonth(ms int64) string {
	return time.UnixMilli(ms).UTC().Format("2006-01")
}

func utcYear(ms int64) string {
	return time.UnixMilli(ms).UTC().Format("2006")
}
