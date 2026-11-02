package job

import (
	"math"
	"strconv"
	"strings"

	"github.com/MaxBernstetter/dagobert/src/execution/internal/store"
	"github.com/MaxBernstetter/dagobert/src/internal/instruments"
)

type KeyFields struct {
	OrderLinkID  string `json:"orderLinkId"`
	Playbook     string `json:"playbook"`
	Symbol       string `json:"symbol"`
	VenueSymbol  string `json:"venueSymbol"`
	CandleClose  int64  `json:"candleClose"`
	Decision     string `json:"decision,omitempty"`
	OrderID      string `json:"orderId,omitempty"`
	OrderStatus  string `json:"orderStatus,omitempty"`
	Side         string `json:"side,omitempty"`
	Qty          string `json:"qty,omitempty"`
	CumExecQty   string `json:"cumExecQty,omitempty"`
	CumExecValue string `json:"cumExecValue,omitempty"`
	AvgPrice     string `json:"avgPrice,omitempty"`
	CumExecFee   string `json:"cumExecFee,omitempty"`
	FeeCurrency  string `json:"feeCurrency,omitempty"`
	MarketUnit   string `json:"marketUnit,omitempty"`
	RetCode      int    `json:"retCode,omitempty"`
	RetMsg       string `json:"retMsg,omitempty"`
}

func journalFrom(key KeyFields) store.Journal {
	return store.Journal{
		OrderLinkID:     key.OrderLinkID,
		Venue:           "bybit",
		VenueSymbol:     key.VenueSymbol,
		Side:            key.Side,
		BaseQty:         key.CumExecQty,
		QuoteQty:        key.CumExecValue,
		Price:           key.AvgPrice,
		Fee:             key.CumExecFee,
		FeeCurrency:     key.FeeCurrency,
		ExchangeOrderID: key.OrderID,
		PriceSource:     "binance",
	}
}

func venueSymbol(symbol string) (string, bool) {
	if inst, ok := instruments.BySeries(symbol); ok && inst.VenueSymbol != "" {
		return inst.VenueSymbol, true
	}
	return "", false
}

// paperVenueSymbol maps a lake series to a venue id for simulated fills.
// Instrument venue mappings win; otherwise the pipeline symbol itself is used.
func paperVenueSymbol(symbol string) (string, bool) {
	if vs, ok := venueSymbol(symbol); ok {
		return vs, true
	}
	inst, ok := instruments.Resolve(symbol)
	if !ok || inst.Series == "" {
		return "", false
	}
	if inst.VenueSymbol != "" {
		return inst.VenueSymbol, true
	}
	return inst.Series, true
}

func orderLinkID(playbook, symbol string, candleClose int64) string {
	return playbook + ":" + symbol + ":" + strconv.FormatInt(candleClose, 10)
}

func formatQty(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) || f <= 0 {
		return ""
	}
	s := strconv.FormatFloat(f, 'f', 8, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" {
		return "0"
	}
	return s
}
