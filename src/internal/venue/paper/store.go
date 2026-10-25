package paper

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/MaxBernstetter/dagobert/src/internal/money"
	"github.com/MaxBernstetter/dagobert/src/internal/venue/bybit"
)

// Enabled when historical seeding or local demo should not call the venue API.
func Enabled() bool {
	return os.Getenv("VENUE_SIMULATE") == "1"
}

type state struct {
	Coins map[string]float64 `json:"coins"`
}

func path(dataDir string) string {
	return filepath.Join(dataDir, "paper_venue.json")
}

// Reset writes demo starting balances (Bybit EU demo–like USDC cash, no BTC).
func Reset(dataDir string) error {
	s := state{Coins: map[string]float64{
		"USDC": 50_000,
		"USDT": 50_000,
		"BTC":  0,
		"ETH":  0,
	}}
	normalize(&s)
	return save(dataDir, s)
}

func load(dataDir string) (state, error) {
	raw, err := os.ReadFile(path(dataDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := Reset(dataDir); err != nil {
				return state{}, err
			}
			return load(dataDir)
		}
		return state{}, err
	}
	var s state
	if err := json.Unmarshal(raw, &s); err != nil {
		return state{}, err
	}
	if s.Coins == nil {
		s.Coins = map[string]float64{}
	}
	normalize(&s)
	return s, nil
}

func normalize(s *state) {
	for cur, amt := range s.Coins {
		s.Coins[cur] = money.RoundCoin(cur, amt)
	}
}

func save(dataDir string, s state) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	normalize(&s)
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path(dataDir), raw, 0o644)
}

// Coin returns one balance as float64 (no string round-trip).
func Coin(dataDir, currency string) (float64, error) {
	s, err := load(dataDir)
	if err != nil {
		return 0, err
	}
	return money.RoundCoin(currency, s.Coins[currency]), nil
}

// QuoteAndBase returns quote and base balances for sizing and execution.
func QuoteAndBase(dataDir, quoteCur, baseCur string) (quoteCash, baseHeld float64, err error) {
	s, err := load(dataDir)
	if err != nil {
		return 0, 0, err
	}
	return money.RoundCoin(quoteCur, s.Coins[quoteCur]), money.RoundCoin(baseCur, s.Coins[baseCur]), nil
}

// Balances returns snapshots compatible with market-action sizing (string field for live Bybit parity).
func Balances(dataDir string) ([]bybit.BalanceSnapshot, error) {
	s, err := load(dataDir)
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	out := make([]bybit.BalanceSnapshot, 0, len(s.Coins))
	for cur, amt := range s.Coins {
		out = append(out, bybit.BalanceSnapshot{
			Currency:        cur,
			CashBalance:     formatBalanceString(cur, amt),
			TransactionTime: now,
		})
	}
	return out, nil
}

func formatBalanceString(currency string, amt float64) string {
	decimals := money.QuoteDecimals
	if currency == "BTC" || currency == "ETH" {
		decimals = money.BaseDecimals
	}
	return strconv.FormatFloat(amt, 'f', decimals, 64)
}

// ApplyBuy spends quoteAmt at price quote per base.
func ApplyBuy(dataDir, quoteCur, baseCur string, quoteAmt, price float64) error {
	quoteAmt = money.RoundQuote(quoteAmt)
	if price <= 0 || quoteAmt <= 0 {
		return errors.New("invalid buy")
	}
	s, err := load(dataDir)
	if err != nil {
		return err
	}
	baseAdd := money.RoundBase(quoteAmt / price)
	q := money.RoundCoin(quoteCur, s.Coins[quoteCur])
	if q < quoteAmt {
		return errors.New("insufficient quote")
	}
	s.Coins[quoteCur] = money.RoundCoin(quoteCur, q-quoteAmt)
	s.Coins[baseCur] = money.RoundCoin(baseCur, s.Coins[baseCur]+baseAdd)
	return save(dataDir, s)
}

// ApplySell sells baseAmt at price.
func ApplySell(dataDir, quoteCur, baseCur string, baseAmt, price float64) error {
	baseAmt = money.RoundBase(baseAmt)
	if price <= 0 || baseAmt <= 0 {
		return errors.New("invalid sell")
	}
	s, err := load(dataDir)
	if err != nil {
		return err
	}
	b := money.RoundCoin(baseCur, s.Coins[baseCur])
	baseAmt = money.RoundBase(math.Min(baseAmt, b))
	if baseAmt <= 0 {
		return errors.New("insufficient base")
	}
	quoteAdd := money.RoundQuote(baseAmt * price)
	s.Coins[baseCur] = money.RoundCoin(baseCur, b-baseAmt)
	s.Coins[quoteCur] = money.RoundCoin(quoteCur, s.Coins[quoteCur]+quoteAdd)
	return save(dataDir, s)
}
