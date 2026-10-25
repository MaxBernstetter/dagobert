package job

import (
	"fmt"
	"math"

	"github.com/MaxBernstetter/dagobert/src/execution/internal/store"
	"github.com/MaxBernstetter/dagobert/src/execution/internal/strategyaction"
	"github.com/MaxBernstetter/dagobert/src/internal/instruments"
	"github.com/MaxBernstetter/dagobert/src/internal/kpi"
	"github.com/MaxBernstetter/dagobert/src/internal/money"
	"github.com/MaxBernstetter/dagobert/src/internal/venue/paper"
	"github.com/MaxBernstetter/dagobert/src/internal/venue/sim"
)

func refreshBalanceSimulated(st *store.DB, dataDir string) error {
	latest, err := paper.Balances(dataDir)
	if err != nil {
		return err
	}
	balRows := make([]store.BalanceRow, 0, len(latest))
	for _, r := range latest {
		balRows = append(balRows, store.BalanceRow{
			Currency:        r.Currency,
			CashBalance:     r.CashBalance,
			TransactionTime: int64(r.TransactionTime),
		})
	}
	return st.ReplaceBalances(balRows)
}

func placeOrderSimulated(
	st *store.DB,
	dataDir string,
	action strategyaction.Action,
	orderLinkID, playbookID, symbol, venueSymbol string,
	candleClose int64,
) (int, int) {
	if action.Size == nil {
		return 1, 30
	}
	inst, ok := instruments.Resolve(symbol)
	if !ok {
		return 1, 10
	}
	barOpen := candleClose - threeHoursMs
	price, err := kpi.BarClose(dataDir, symbol, barOpen)
	if err != nil {
		return 1, 30
	}

	key := KeyFields{
		OrderLinkID: orderLinkID,
		Playbook:    playbookID,
		Symbol:      symbol,
		VenueSymbol: venueSymbol,
		CandleClose: candleClose,
	}
	orderID := fmt.Sprintf("sim-%d", candleClose)
	costs := sim.FromEnv()

	switch action.Decision {
	case "buy":
		quoteSpend := money.RoundQuote(*action.Size)
		baseQty, effPrice, fee, err := sim.ApplyBuyFile(dataDir, inst.Quote, inst.Base, quoteSpend, price, costs)
		if err != nil {
			rej := key
			rej.RetCode = 1
			rej.RetMsg = err.Error()
			if err := st.InsertOrderResult(orderLinkID, store.KindRejection, rej); err != nil {
				return 1, 30
			}
			return 1, 40
		}
		fill := key
		fill.OrderID = orderID
		fill.OrderStatus = "Filled"
		fill.Side = "Buy"
		fill.Qty = formatQty(quoteSpend)
		fill.CumExecQty = formatQty(baseQty)
		fill.CumExecValue = formatQty(quoteSpend)
		fill.AvgPrice = formatQty(effPrice)
		fill.MarketUnit = "quoteCoin"
		fill.CumExecFee = formatQty(fee)
		fill.FeeCurrency = inst.Quote
		if err := st.InsertOrderResult(orderLinkID, store.KindFill, fill); err != nil {
			return 1, 30
		}
		_ = st.UpsertJournal(journalFrom(fill))
	case "sell":
		held, _ := paper.Coin(dataDir, inst.Base)
		baseQty := money.FloorBase(math.Min(*action.Size, held))
		quoteVal, effPrice, fee, err := sim.ApplySellFile(dataDir, inst.Quote, inst.Base, baseQty, price, costs)
		if err != nil {
			rej := key
			rej.RetCode = 1
			rej.RetMsg = err.Error()
			if err := st.InsertOrderResult(orderLinkID, store.KindRejection, rej); err != nil {
				return 1, 30
			}
			return 1, 40
		}
		fill := key
		fill.OrderID = orderID
		fill.OrderStatus = "Filled"
		fill.Side = "Sell"
		fill.Qty = formatQty(baseQty)
		fill.CumExecQty = formatQty(baseQty)
		fill.CumExecValue = formatQty(quoteVal)
		fill.AvgPrice = formatQty(effPrice)
		fill.MarketUnit = "baseCoin"
		fill.CumExecFee = formatQty(fee)
		fill.FeeCurrency = inst.Quote
		if err := st.InsertOrderResult(orderLinkID, store.KindFill, fill); err != nil {
			return 1, 30
		}
		_ = st.UpsertJournal(journalFrom(fill))
	default:
		return 1, 30
	}

	if err := refreshBalanceSimulated(st, dataDir); err != nil {
		return 1, 30
	}
	return 0, 0
}

func runSimulatedNoTrade(st *store.DB, dataDir string, body KeyFields) (int, int) {
	if err := refreshBalanceSimulated(st, dataDir); err != nil {
		return 1, 30
	}
	if err := st.InsertOrderResult(body.OrderLinkID, store.KindNoTrade, body); err != nil {
		return 1, 30
	}
	return 0, 2
}
