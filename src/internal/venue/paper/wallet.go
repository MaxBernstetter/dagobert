package paper

import (
	"errors"
	"math"

	"github.com/MaxBernstetter/trading-copilot/src/internal/money"
)

// Wallet is an in-memory paper venue (no per-fill disk I/O).
type Wallet struct {
	Coins map[string]float64
}

// LoadWallet reads paper_venue.json into memory.
func LoadWallet(dataDir string) (Wallet, error) {
	s, err := load(dataDir)
	if err != nil {
		return Wallet{}, err
	}
	return Wallet{Coins: s.Coins}, nil
}

// Save writes the wallet to paper_venue.json once.
func (w *Wallet) Save(dataDir string) error {
	s := state{Coins: w.Coins}
	return save(dataDir, s)
}

// QuoteAndBase returns balances for sizing.
func (w *Wallet) QuoteAndBase(quoteCur, baseCur string) (float64, float64) {
	return money.RoundCoin(quoteCur, w.Coins[quoteCur]), money.RoundCoin(baseCur, w.Coins[baseCur])
}

// ApplyBuy spends quote at price (quote per base).
func (w *Wallet) ApplyBuy(quoteCur, baseCur string, quoteAmt, price float64) error {
	quoteAmt = money.RoundQuote(quoteAmt)
	if price <= 0 || quoteAmt <= 0 {
		return errors.New("invalid buy")
	}
	baseAdd := money.RoundBase(quoteAmt / price)
	q := money.RoundCoin(quoteCur, w.Coins[quoteCur])
	if q < quoteAmt {
		return errors.New("insufficient quote")
	}
	w.Coins[quoteCur] = money.RoundCoin(quoteCur, q-quoteAmt)
	w.Coins[baseCur] = money.RoundCoin(baseCur, w.Coins[baseCur]+baseAdd)
	return nil
}

// ApplySell sells base at price.
func (w *Wallet) ApplySell(quoteCur, baseCur string, baseAmt, price float64) error {
	baseAmt = money.RoundBase(baseAmt)
	if price <= 0 || baseAmt <= 0 {
		return errors.New("invalid sell")
	}
	b := money.RoundCoin(baseCur, w.Coins[baseCur])
	baseAmt = money.RoundBase(math.Min(baseAmt, b))
	if baseAmt <= 0 {
		return errors.New("insufficient base")
	}
	quoteAdd := money.RoundQuote(baseAmt * price)
	w.Coins[baseCur] = money.RoundCoin(baseCur, b-baseAmt)
	w.Coins[quoteCur] = money.RoundCoin(quoteCur, w.Coins[quoteCur]+quoteAdd)
	return nil
}

// BalanceSnapshots for execution.sqlite balance table.
func (w *Wallet) BalanceSnapshots() []struct {
	Currency    string
	CashBalance string
} {
	out := make([]struct {
		Currency    string
		CashBalance string
	}, 0, len(w.Coins))
	for cur, amt := range w.Coins {
		out = append(out, struct {
			Currency    string
			CashBalance string
		}{Currency: cur, CashBalance: formatBalanceString(cur, amt)})
	}
	return out
}
