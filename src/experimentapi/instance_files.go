package experimentapi

import (
	"errors"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/MaxBernstetter/dagobert/src/calculation-performance"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"github.com/MaxBernstetter/dagobert/src/internal/instanceparquet"
)

// The instance Parquet files (orders, journals, fills, trade outcomes) are read straight from the
// instance workspace. A missing file is a valid empty result for orders and journals; callers
// check errors.Is(err, fs.ErrNotExist) to decide between the Parquet and the legacy SQLite source.

// activityRowsFromOrders maps order rows to the activity rows the UI renders, newest candle first.
func activityRowsFromOrders(orders []instanceparquet.Order) []activityRow {
	rows := make([]activityRow, 0, len(orders))
	for _, o := range orders {
		row := activityRow{
			OrderLinkID:  o.OrderLinkID,
			Playbook:     o.Playbook,
			Symbol:       o.Symbol,
			CandleClose:  o.CandleClose,
			Kind:         o.Kind,
			Decision:     o.Decision,
			OrderID:      o.OrderID,
			OrderStatus:  o.OrderStatus,
			VenueSymbol:  optionalString(o.VenueSymbol),
			Side:         o.Side,
			Qty:          o.Qty,
			CumExecQty:   o.CumExecQty,
			CumExecValue: o.CumExecValue,
			AvgPrice:     o.AvgPrice,
			MarketUnit:   o.MarketUnit,
			RetMsg:       o.RetMsg,
		}
		if o.RetCode != nil {
			code := float64(*o.RetCode)
			row.RetCode = &code
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].CandleClose > rows[j].CandleClose })
	return rows
}

// optionalString returns a pointer to s, or nil when s is empty (the JSON side omits it).
func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// journalRowsFromParquet maps journal rows to the wire shape, in simulation order.
func journalRowsFromParquet(journals []instanceparquet.Journal) []journalRow {
	out := make([]journalRow, 0, len(journals))
	for _, j := range journals {
		out = append(out, journalRow{
			OrderLinkID: j.OrderLinkID,
			Venue:       j.Venue,
			VenueSymbol: j.VenueSymbol,
			Side:        j.Side,
			BaseQty:     j.BaseQty,
			QuoteQty:    j.QuoteQty,
			Price:       j.Price,
			Fee:         j.Fee,
			FeeCurrency: j.FeeCurrency,
			PriceSource: j.PriceSource,
		})
	}
	return out
}

// journalsFromFillOrders derives the journal rows from the filled orders, for an instance without a
// journal file. It is the same derivation the SQLite fallback used.
func journalsFromFillOrders(orders []instanceparquet.Order) []journalRow {
	out := []journalRow{}
	for _, o := range orders {
		if o.Kind != "fill" {
			continue
		}
		out = append(out, journalRow{
			OrderLinkID: o.OrderLinkID,
			Venue:       "bybit",
			VenueSymbol: o.VenueSymbol,
			Side:        derefString(o.Side),
			BaseQty:     derefString(o.CumExecQty),
			QuoteQty:    derefString(o.CumExecValue),
			Price:       derefString(o.AvgPrice),
			Fee:         derefString(o.CumExecFee),
			FeeCurrency: defaultString(derefString(o.FeeCurrency), "USDC"),
			PriceSource: "binance",
		})
	}
	return out
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// chartFill is one fill as the equity curve and the markers read it.
type chartFill struct {
	closeMs int64
	side    string
	base    float64
	quote   float64
	price   float64
}

func chartFillsFromParquet(fills []instanceparquet.Fill) []chartFill {
	out := make([]chartFill, 0, len(fills))
	for _, f := range fills {
		out = append(out, chartFill{closeMs: f.TsMs, side: f.Side, base: f.BaseQty, quote: f.QuoteQty, price: f.Price})
	}
	return out
}

// instanceTradeOutcomes returns the closed trades of an instance from trade_outcomes.parquet. A
// missing file is no trades.
func instanceTradeOutcomes(inst experimentdb.ExperimentInstance) ([]performance.ClosedTrade, error) {
	outcomes, err := instanceparquet.ReadTradeOutcomes(instanceWorkspace(inst))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return closedTrades(outcomes), nil
}

// closedTrades maps trade outcome rows to the performance input.
func closedTrades(outcomes []instanceparquet.TradeOutcome) []performance.ClosedTrade {
	closed := make([]performance.ClosedTrade, 0, len(outcomes))
	for _, tr := range outcomes {
		ct := performance.ClosedTrade{Quote: float64(tr.Quote), Side: int(tr.Side)}
		if tr.Percent != nil {
			pct := float64(*tr.Percent)
			ct.Percent = &pct
		}
		closed = append(closed, ct)
	}
	return closed
}

// equityFromFills is the mark-to-market curve: a start point one second before the first fill, then
// cash plus base times the fill price after every fill.
func equityFromFills(fills []chartFill, startCash float64) []*v1.ChartPoint {
	sort.Slice(fills, func(i, j int) bool { return fills[i].closeMs < fills[j].closeMs })
	if len(fills) == 0 {
		return nil
	}
	cash := startCash
	base := 0.0
	points := []*v1.ChartPoint{{TimeSec: fills[0].closeMs/1000 - 1, Value: startCash}}
	for _, f := range fills {
		switch strings.ToLower(f.side) {
		case "buy":
			cash -= f.quote
			base += f.base
		case "sell":
			cash += f.quote
			base -= f.base
		}
		points = append(points, &v1.ChartPoint{TimeSec: f.closeMs / 1000, Value: cash + base*f.price})
	}
	return points
}

// markersFromFills puts a marker on every buy and sell fill.
func markersFromFills(fills []chartFill) []*v1.ChartMarker {
	var out []*v1.ChartMarker
	for _, f := range fills {
		if f.closeMs <= 0 {
			continue
		}
		var side v1.ChartSide
		switch strings.ToLower(f.side) {
		case "buy":
			side = v1.ChartSide_CHART_SIDE_BUY
		case "sell":
			side = v1.ChartSide_CHART_SIDE_SELL
		default:
			continue
		}
		out = append(out, &v1.ChartMarker{TimeSec: f.closeMs / 1000, Side: side})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TimeSec < out[j].TimeSec })
	return out
}

// journalFills converts legacy journal rows to chart fills; rows without a candle close or with
// unparsable quantities are skipped.
func journalFills(journals []journalRow) []chartFill {
	var fills []chartFill
	for _, j := range journals {
		ms := candleCloseFromLink(j.OrderLinkID)
		if ms == 0 {
			continue
		}
		base, err1 := strconv.ParseFloat(j.BaseQty, 64)
		quote, err2 := strconv.ParseFloat(j.QuoteQty, 64)
		price, err3 := strconv.ParseFloat(j.Price, 64)
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		fills = append(fills, chartFill{closeMs: ms, side: strings.ToLower(j.Side), base: base, quote: quote, price: price})
	}
	return fills
}
