package sim

import (
	"errors"
	"math"

	"github.com/MaxBernstetter/dagobert/src/internal/money"
	"github.com/MaxBernstetter/dagobert/src/internal/venue/paper"
)

// ApplyBuy spends quote at mid using fee/slippage model.
func ApplyBuy(w *paper.Wallet, quoteCur, baseCur string, quoteSpend, mid float64, m Model) (baseQty, effPrice, fee float64, err error) {
	baseQty, effPrice, fee, _, err = m.BuyFill(quoteSpend, mid)
	if err != nil {
		return 0, 0, 0, err
	}
	quoteSpend = money.RoundQuote(quoteSpend)
	q := money.RoundCoin(quoteCur, w.Coins[quoteCur])
	if q < quoteSpend {
		return 0, 0, 0, errors.New("insufficient quote")
	}
	w.Coins[quoteCur] = money.RoundCoin(quoteCur, q-quoteSpend)
	w.Coins[baseCur] = money.RoundCoin(baseCur, w.Coins[baseCur]+baseQty)
	return baseQty, effPrice, fee, nil
}

// ApplySell sells base at mid using fee/slippage model.
func ApplySell(w *paper.Wallet, quoteCur, baseCur string, baseAmt, mid float64, m Model) (quoteCredit, effPrice, fee float64, err error) {
	b := money.RoundCoin(baseCur, w.Coins[baseCur])
	baseAmt = money.RoundBase(math.Min(baseAmt, b))
	if baseAmt <= 0 {
		return 0, 0, 0, errors.New("insufficient base")
	}
	quoteCredit, effPrice, fee, err = m.SellFill(baseAmt, mid)
	if err != nil {
		return 0, 0, 0, err
	}
	w.Coins[baseCur] = money.RoundCoin(baseCur, b-baseAmt)
	w.Coins[quoteCur] = money.RoundCoin(quoteCur, w.Coins[quoteCur]+quoteCredit)
	return quoteCredit, effPrice, fee, nil
}

// ApplyBuyFile is disk-backed paper wallet (per-bar execution job).
func ApplyBuyFile(dataDir, quoteCur, baseCur string, quoteSpend, mid float64, m Model) (baseQty, effPrice, fee float64, err error) {
	w, err := paper.LoadWallet(dataDir)
	if err != nil {
		return 0, 0, 0, err
	}
	baseQty, effPrice, fee, err = ApplyBuy(&w, quoteCur, baseCur, quoteSpend, mid, m)
	if err != nil {
		return 0, 0, 0, err
	}
	return baseQty, effPrice, fee, w.Save(dataDir)
}

// ApplySellFile is disk-backed paper wallet (per-bar execution job).
func ApplySellFile(dataDir, quoteCur, baseCur string, baseAmt, mid float64, m Model) (quoteCredit, effPrice, fee float64, err error) {
	w, err := paper.LoadWallet(dataDir)
	if err != nil {
		return 0, 0, 0, err
	}
	quoteCredit, effPrice, fee, err = ApplySell(&w, quoteCur, baseCur, baseAmt, mid, m)
	if err != nil {
		return 0, 0, 0, err
	}
	return quoteCredit, effPrice, fee, w.Save(dataDir)
}
