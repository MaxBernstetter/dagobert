package runner

import (
	"fmt"

	"github.com/MaxBernstetter/dagobert/src/execution/internal/job"
	"github.com/MaxBernstetter/dagobert/src/execution/internal/store"
	"github.com/MaxBernstetter/dagobert/src/internal/instruments"
	"github.com/MaxBernstetter/dagobert/src/internal/money"
	"github.com/MaxBernstetter/dagobert/src/internal/venue/paper"
	"github.com/MaxBernstetter/dagobert/src/internal/venue/sim"
)

// SimBarResult is one simulated execution step for batched replay.
type SimBarResult struct {
	Orders   []store.OrderResultRow
	Journals []store.Journal
	Position string
}

// SimulateBar applies one sized decision at bar close using in-memory paper wallet.
func SimulateBar(wallet *paper.Wallet, playbookID, symbol string, candleClose int64, decision string, size *float64, price float64, position string) (SimBarResult, error) {
	return SimulateBarWithModel(wallet, playbookID, symbol, candleClose, decision, size, price, position, sim.FromEnv(), false)
}

// SimulateBarWithModel is SimulateBar with a preloaded cost model.
// When skipNoTrade is true, no-trade bars produce empty Orders (slim experiment path).
func SimulateBarWithModel(wallet *paper.Wallet, playbookID, symbol string, candleClose int64, decision string, size *float64, price float64, position string, costs sim.Model, skipNoTrade bool) (SimBarResult, error) {
	inst, ok := instruments.Resolve(symbol)
	if !ok {
		return SimBarResult{}, fmt.Errorf("unknown symbol %s", symbol)
	}
	venueSymbol, ok := job.VenueSymbolForSeries(symbol)
	if !ok {
		return SimBarResult{}, fmt.Errorf("no venue symbol")
	}
	link := job.OrderLinkID(playbookID, symbol, candleClose)
	key := job.KeyFields{
		OrderLinkID: link,
		Playbook:    playbookID,
		Symbol:      symbol,
		VenueSymbol: venueSymbol,
		CandleClose: candleClose,
		Decision:    decision,
	}
	out := SimBarResult{Position: position}

	switch decision {
	case "no-trade":
		if !skipNoTrade {
			out.Orders = []store.OrderResultRow{{OrderLinkID: link, Kind: store.KindNoTrade, Body: key}}
		}
		return out, nil
	case "buy", "sell":
		if size == nil {
			return SimBarResult{}, fmt.Errorf("missing size for %s", decision)
		}
		orderID := fmt.Sprintf("sim-%d", candleClose)
		switch decision {
		case "buy":
			quoteSpend := money.RoundQuote(*size)
			baseQty, effPrice, fee, err := sim.ApplyBuy(wallet, inst.Quote, inst.Base, quoteSpend, price, costs)
			if err != nil {
				rej := key
				rej.RetCode = 1
				rej.RetMsg = err.Error()
				out.Orders = []store.OrderResultRow{{OrderLinkID: link, Kind: store.KindRejection, Body: rej}}
				return out, nil
			}
			fill := key
			fill.OrderID = orderID
			fill.OrderStatus = "Filled"
			fill.Side = "Buy"
			fill.Qty = job.FormatOrderQty(quoteSpend)
			fill.CumExecQty = job.FormatOrderQty(baseQty)
			fill.CumExecValue = job.FormatOrderQty(quoteSpend)
			fill.AvgPrice = job.FormatOrderQty(effPrice)
			fill.MarketUnit = "quoteCoin"
			fill.CumExecFee = job.FormatOrderQty(fee)
			fill.FeeCurrency = inst.Quote
			out.Orders = []store.OrderResultRow{{OrderLinkID: link, Kind: store.KindFill, Body: fill}}
			out.Journals = []store.Journal{job.JournalFromFill(fill)}
			out.Position = "long"
			return out, nil
		case "sell":
			_, baseHeld := wallet.QuoteAndBase(inst.Quote, inst.Base)
			baseAmt := money.FloorBase(minFloat(*size, baseHeld))
			quoteVal, effPrice, fee, err := sim.ApplySell(wallet, inst.Quote, inst.Base, baseAmt, price, costs)
			if err != nil {
				rej := key
				rej.RetCode = 1
				rej.RetMsg = err.Error()
				out.Orders = []store.OrderResultRow{{OrderLinkID: link, Kind: store.KindRejection, Body: rej}}
				return out, nil
			}
			fill := key
			fill.OrderID = orderID
			fill.OrderStatus = "Filled"
			fill.Side = "Sell"
			fill.Qty = job.FormatOrderQty(baseAmt)
			fill.CumExecQty = job.FormatOrderQty(baseAmt)
			fill.CumExecValue = job.FormatOrderQty(quoteVal)
			fill.AvgPrice = job.FormatOrderQty(effPrice)
			fill.MarketUnit = "baseCoin"
			fill.CumExecFee = job.FormatOrderQty(fee)
			fill.FeeCurrency = inst.Quote
			out.Orders = []store.OrderResultRow{{OrderLinkID: link, Kind: store.KindFill, Body: fill}}
			out.Journals = []store.Journal{job.JournalFromFill(fill)}
			out.Position = "flat"
			return out, nil
		}
	}
	return SimBarResult{}, fmt.Errorf("unknown decision %q", decision)
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
