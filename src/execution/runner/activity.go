package runner

import (
	"github.com/MaxBernstetter/dagobert/src/execution/internal/job"
	"github.com/MaxBernstetter/dagobert/src/execution/internal/store"
	"github.com/MaxBernstetter/dagobert/src/internal/instanceparquet"
)

// ActivityFromStep converts one simulated step to Parquet rows. The ordinals start at the given
// counts, which are the rows already produced by earlier steps.
func ActivityFromStep(step SimBarResult, orderBase, journalBase int32) ([]instanceparquet.Order, []instanceparquet.Journal) {
	orders := make([]instanceparquet.Order, 0, len(step.Orders))
	for i, o := range step.Orders {
		orders = append(orders, orderRow(orderBase+int32(i), o))
	}
	journals := make([]instanceparquet.Journal, 0, len(step.Journals))
	for i, j := range step.Journals {
		journals = append(journals, journalRow(journalBase+int32(i), j))
	}
	return orders, journals
}

// orderRow maps an order result onto the activity columns. Empty and zero values become null,
// as the JSON body omitted them (omitempty).
func orderRow(ordinal int32, o store.OrderResultRow) instanceparquet.Order {
	row := instanceparquet.Order{Ordinal: ordinal, OrderLinkID: o.OrderLinkID, Kind: o.Kind}
	key, ok := o.Body.(job.KeyFields)
	if !ok {
		return row
	}
	row.OrderLinkID = key.OrderLinkID
	row.Playbook = key.Playbook
	row.Symbol = key.Symbol
	row.VenueSymbol = key.VenueSymbol
	row.CandleClose = key.CandleClose
	row.Decision = optString(key.Decision)
	row.OrderID = optString(key.OrderID)
	row.OrderStatus = optString(key.OrderStatus)
	row.Side = optString(key.Side)
	row.Qty = optString(key.Qty)
	row.CumExecQty = optString(key.CumExecQty)
	row.CumExecValue = optString(key.CumExecValue)
	row.AvgPrice = optString(key.AvgPrice)
	row.CumExecFee = optString(key.CumExecFee)
	row.FeeCurrency = optString(key.FeeCurrency)
	row.MarketUnit = optString(key.MarketUnit)
	row.RetMsg = optString(key.RetMsg)
	if key.RetCode != 0 {
		code := int32(key.RetCode)
		row.RetCode = &code
	}
	return row
}

func journalRow(ordinal int32, j store.Journal) instanceparquet.Journal {
	return instanceparquet.Journal{
		Ordinal:         ordinal,
		OrderLinkID:     j.OrderLinkID,
		Venue:           j.Venue,
		VenueSymbol:     j.VenueSymbol,
		Side:            j.Side,
		BaseQty:         j.BaseQty,
		QuoteQty:        j.QuoteQty,
		Price:           j.Price,
		Fee:             j.Fee,
		FeeCurrency:     j.FeeCurrency,
		ExchangeOrderID: j.ExchangeOrderID,
		PriceSource:     j.PriceSource,
	}
}

func optString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
