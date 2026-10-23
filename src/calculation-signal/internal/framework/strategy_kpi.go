package framework

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	strategyKPIBarMS          = 3 * 60 * 60 * 1000
	strategyKPIPeriodsPerYear = 365.25 * 24 / 3
	defaultStrategyStartCash  = 50_000
)

// StrategyKPIDef describes one playbook performance metric (see playbooks/strategy-kpi/*/README.md).
type StrategyKPIDef struct {
	Slug   string `json:"slug"`
	Title  string `json:"title"`
	Label  string `json:"label"`
	Unit   string `json:"unit"`
	Format string `json:"format"`
}

// StrategyKPIResult is one computed KPI value for a playbook and time range.
type StrategyKPIResult struct {
	Def   StrategyKPIDef     `json:"def"`
	Value float64            `json:"value"`
	Extra map[string]float64 `json:"extra,omitempty"`
}

// StrategyKPISet is the full KPI snapshot for one evaluation window.
type StrategyKPISet struct {
	Playbook string              `json:"playbook"`
	Symbol   string              `json:"symbol"`
	FromMs   int64               `json:"fromMs"`
	ToMs     int64               `json:"toMs"`
	Results  []StrategyKPIResult `json:"results"`
}

// StrategyKPIFill is one execution fill used for equity replay.
type StrategyKPIFill struct {
	CandleCloseMs int64
	Side          string
	BaseQty       float64
	QuoteQty      float64
	Price         float64
}

// StrategyKPIBar is one mark-to-market sample at 3h candle close.
type StrategyKPIBar struct {
	CloseMs int64
	Close   float64
}

// StrategyKPIRegistry returns the five standard playbook KPI definitions.
func StrategyKPIRegistry() []StrategyKPIDef {
	return []StrategyKPIDef{
		{Slug: "cagr", Title: "CAGR (net)", Label: "CAGR", Unit: "ratio", Format: "percent"},
		{Slug: "max-drawdown", Title: "Maximum drawdown", Label: "Max DD", Unit: "ratio", Format: "percent"},
		{Slug: "sharpe", Title: "Sharpe ratio", Label: "Sharpe", Unit: "ratio", Format: "decimal"},
		{Slug: "active-return", Title: "Active return vs benchmark", Label: "vs B&H", Unit: "ratio", Format: "percent-pts"},
		{Slug: "sortino", Title: "Sortino ratio", Label: "Sortino", Unit: "ratio", Format: "decimal"},
	}
}

// ComputeStrategyKPIs loads fills and bar marks from dataDir and computes KPIs for [fromMs, toMs].
// When fromMs or toMs is zero, the range spans all available fills and marks.
func ComputeStrategyKPIs(dataDir, playbook, symbol string, fromMs, toMs int64) (StrategyKPISet, error) {
	fills, err := LoadStrategyKPIFills(dataDir, playbook)
	if err != nil {
		return StrategyKPISet{}, err
	}
	if len(fills) == 0 {
		return StrategyKPISet{Playbook: playbook, Symbol: symbol, Results: nil}, nil
	}

	if fromMs <= 0 {
		fromMs = fills[0].CandleCloseMs
	}
	if toMs <= 0 {
		toMs = fills[len(fills)-1].CandleCloseMs
	}
	if fromMs > toMs {
		return StrategyKPISet{}, errors.New("invalid kpi range")
	}

	bars, err := LoadStrategyKPIBars(dataDir, symbol, fromMs, toMs)
	if err != nil {
		return StrategyKPISet{}, err
	}
	if len(bars) < 2 {
		return StrategyKPISet{Playbook: playbook, Symbol: symbol, FromMs: fromMs, ToMs: toMs, Results: nil}, nil
	}

	portfolio, bench := ReplayEquityCurves(fills, bars, defaultStrategyStartCash)
	if len(portfolio) < 2 {
		return StrategyKPISet{Playbook: playbook, Symbol: symbol, FromMs: fromMs, ToMs: toMs, Results: nil}, nil
	}

	results := ComputeStrategyKPIValues(portfolio, bench, fromMs, toMs)
	return StrategyKPISet{
		Playbook: playbook,
		Symbol:   symbol,
		FromMs:   fromMs,
		ToMs:     toMs,
		Results:  results,
	}, nil
}

// LoadStrategyKPIFills reads fill_journal rows for a playbook.
func LoadStrategyKPIFills(dataDir, playbook string) ([]StrategyKPIFill, error) {
	path := filepath.Join(dataDir, "execution.sqlite")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	prefix := playbook + ":"
	rows, err := db.Query(
		`SELECT order_link_id, side, base_qty, quote_qty, price FROM fill_journal WHERE order_link_id LIKE ?`,
		prefix+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []StrategyKPIFill
	for rows.Next() {
		var link, side, baseS, quoteS, priceS string
		if err := rows.Scan(&link, &side, &baseS, &quoteS, &priceS); err != nil {
			return nil, err
		}
		closeMs, ok := candleCloseFromOrderLink(link, playbook)
		if !ok {
			continue
		}
		baseQty, _ := strconv.ParseFloat(strings.TrimSpace(baseS), 64)
		quoteQty, _ := strconv.ParseFloat(strings.TrimSpace(quoteS), 64)
		price, _ := strconv.ParseFloat(strings.TrimSpace(priceS), 64)
		out = append(out, StrategyKPIFill{
			CandleCloseMs: closeMs,
			Side:          strings.ToLower(side),
			BaseQty:       baseQty,
			QuoteQty:      quoteQty,
			Price:         price,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CandleCloseMs < out[j].CandleCloseMs })
	return out, nil
}

func candleCloseFromOrderLink(link, playbook string) (int64, bool) {
	prefix := playbook + ":"
	last := strings.LastIndex(link, ":")
	if last <= 0 {
		return 0, false
	}
	if !strings.HasPrefix(link, prefix) {
		return 0, false
	}
	ms, err := strconv.ParseInt(link[last+1:], 10, 64)
	if err != nil || ms <= 0 {
		return 0, false
	}
	return ms, true
}

// LoadStrategyKPIBars loads 3h bar close prices for symbol within candle-close bounds.
func LoadStrategyKPIBars(dataDir, symbol string, fromCloseMs, toCloseMs int64) ([]StrategyKPIBar, error) {
	path := filepath.Join(dataDir, "technical_indicator.sqlite")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	openMin := fromCloseMs - strategyKPIBarMS
	openMax := toCloseMs - strategyKPIBarMS
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(
		`SELECT bar_open, body FROM kpi_revision WHERE symbol = ? AND algo = 'v1' AND bar_open >= ? AND bar_open <= ? ORDER BY bar_open`,
		symbol, openMin, openMax,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []StrategyKPIBar
	for rows.Next() {
		var barOpen int64
		var body string
		if err := rows.Scan(&barOpen, &body); err != nil {
			return nil, err
		}
		closeMs := barOpen + strategyKPIBarMS
		if closeMs < fromCloseMs || closeMs > toCloseMs {
			continue
		}
		closePx, err := parseBarClose(body)
		if err != nil {
			continue
		}
		out = append(out, StrategyKPIBar{CloseMs: closeMs, Close: closePx})
	}
	return out, rows.Err()
}

func parseBarClose(body string) (float64, error) {
	var row struct {
		Close float64 `json:"close"`
	}
	if err := json.Unmarshal([]byte(body), &row); err != nil {
		return 0, err
	}
	if row.Close <= 0 || math.IsNaN(row.Close) {
		return 0, fmt.Errorf("invalid close")
	}
	return row.Close, nil
}

type equityPoint struct {
	CloseMs int64
	Value   float64
}

// ReplayEquityCurves builds portfolio and buy-and-hold benchmark equity at each bar close.
func ReplayEquityCurves(fills []StrategyKPIFill, bars []StrategyKPIBar, startCash float64) ([]equityPoint, []equityPoint) {
	fillAt := map[int64][]StrategyKPIFill{}
	for _, f := range fills {
		fillAt[f.CandleCloseMs] = append(fillAt[f.CandleCloseMs], f)
	}

	cash := startCash
	base := 0.0
	var portfolio []equityPoint

	benchCash := startCash
	benchBase := 0.0
	var bench []equityPoint
	benchInitialized := false

	for _, bar := range bars {
		for _, f := range fillAt[bar.CloseMs] {
			switch f.Side {
			case "buy":
				cash -= f.QuoteQty
				base += f.BaseQty
			case "sell":
				cash += f.QuoteQty
				base -= f.BaseQty
			}
		}
		portfolio = append(portfolio, equityPoint{CloseMs: bar.CloseMs, Value: cash + base*bar.Close})

		if !benchInitialized {
			benchBase = benchCash / bar.Close
			benchCash = 0
			benchInitialized = true
		}
		bench = append(bench, equityPoint{CloseMs: bar.CloseMs, Value: benchCash + benchBase*bar.Close})
	}
	return portfolio, bench
}

// ComputeStrategyKPIValues derives the five KPI results from equity series.
func ComputeStrategyKPIValues(portfolio, benchmark []equityPoint, fromMs, toMs int64) []StrategyKPIResult {
	defs := StrategyKPIRegistry()
	if len(portfolio) < 2 {
		return nil
	}
	startV := portfolio[0].Value
	endV := portfolio[len(portfolio)-1].Value
	if startV <= 0 {
		return nil
	}

	years := float64(toMs-fromMs) / (365.25 * 24 * float64(time.Hour/time.Millisecond))
	if years <= 0 {
		years = float64(portfolio[len(portfolio)-1].CloseMs-portfolio[0].CloseMs) / (365.25 * 24 * float64(time.Hour/time.Millisecond))
	}

	out := make([]StrategyKPIResult, 0, len(defs))
	for _, d := range defs {
		switch d.Slug {
		case "cagr":
			if v, ok := computeCAGR(startV, endV, years); ok {
				out = append(out, StrategyKPIResult{Def: d, Value: v})
			}
		case "max-drawdown":
			if v, extra, ok := computeMaxDrawdown(portfolio); ok {
				out = append(out, StrategyKPIResult{Def: d, Value: v, Extra: extra})
			}
		case "sharpe":
			if v, ok := computeSharpe(portfolio); ok {
				out = append(out, StrategyKPIResult{Def: d, Value: v})
			}
		case "active-return":
			if len(benchmark) >= 2 && benchmark[0].Value > 0 {
				stratRet := endV/startV - 1
				benchRet := benchmark[len(benchmark)-1].Value/benchmark[0].Value - 1
				out = append(out, StrategyKPIResult{Def: d, Value: stratRet - benchRet})
			}
		case "sortino":
			if v, ok := computeSortino(portfolio); ok {
				out = append(out, StrategyKPIResult{Def: d, Value: v})
			}
		}
	}
	return out
}

func computeCAGR(start, end, years float64) (float64, bool) {
	if start <= 0 || end <= 0 || years <= 0 {
		return 0, false
	}
	return math.Pow(end/start, 1/years) - 1, true
}

func computeMaxDrawdown(curve []equityPoint) (float64, map[string]float64, bool) {
	peak := curve[0].Value
	maxDD := 0.0
	var ddPeakValue float64
	var troughMs int64

	for _, p := range curve {
		if p.Value > peak {
			peak = p.Value
		}
		if peak > 0 {
			dd := (peak - p.Value) / peak
			if dd > maxDD {
				maxDD = dd
				ddPeakValue = peak
				troughMs = p.CloseMs
			}
		}
	}

	extra := map[string]float64{}
	for _, p := range curve {
		if p.CloseMs <= troughMs {
			continue
		}
		if p.Value >= ddPeakValue {
			days := float64(p.CloseMs-troughMs) / float64(24*time.Hour/time.Millisecond)
			extra["recovery_days"] = days
			break
		}
	}
	return maxDD, extra, true
}

func periodicReturns(curve []equityPoint) []float64 {
	out := make([]float64, 0, len(curve)-1)
	for i := 1; i < len(curve); i++ {
		prev := curve[i-1].Value
		if prev <= 0 {
			continue
		}
		out = append(out, curve[i].Value/prev-1)
	}
	return out
}

func computeSharpe(curve []equityPoint) (float64, bool) {
	rets := periodicReturns(curve)
	if len(rets) < 2 {
		return 0, false
	}
	mean, std := meanStd(rets)
	if std == 0 {
		return 0, false
	}
	return (mean / std) * math.Sqrt(strategyKPIPeriodsPerYear), true
}

func computeSortino(curve []equityPoint) (float64, bool) {
	rets := periodicReturns(curve)
	if len(rets) < 2 {
		return 0, false
	}
	mean := 0.0
	for _, r := range rets {
		mean += r
	}
	mean /= float64(len(rets))

	var downSum float64
	var downN int
	for _, r := range rets {
		if r < 0 {
			downSum += r * r
			downN++
		}
	}
	if downN == 0 {
		return 0, false
	}
	downDev := math.Sqrt(downSum / float64(downN))
	if downDev == 0 {
		return 0, false
	}
	return (mean / downDev) * math.Sqrt(strategyKPIPeriodsPerYear), true
}

func meanStd(v []float64) (float64, float64) {
	if len(v) == 0 {
		return 0, 0
	}
	mean := 0.0
	for _, x := range v {
		mean += x
	}
	mean /= float64(len(v))
	var sum float64
	for _, x := range v {
		d := x - mean
		sum += d * d
	}
	return mean, math.Sqrt(sum / float64(len(v)))
}
