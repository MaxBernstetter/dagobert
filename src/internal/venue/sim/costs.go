package sim

import (
	"errors"
	"math"
	"os"
	"strconv"

	"github.com/MaxBernstetter/dagobert/src/internal/money"
)

// Model is execution costs vs the bar mid/close (basis points).
// SpreadBps is the full bid–ask width; half is paid on buy and half on sell.
// SlippageBps is additional adverse movement (impact) on top of half-spread.
// FeeBps is exchange fee on quote notional at the effective price.
type Model struct {
	SpreadBps   float64
	SlippageBps float64
	FeeBps      float64
}

// FromEnv reads SIM_SPREAD_BPS, SIM_SLIPPAGE_BPS, and SIM_FEE_BPS (0 when unset).
func FromEnv() Model {
	return Model{
		SpreadBps:   envFloat("SIM_SPREAD_BPS"),
		SlippageBps: envFloat("SIM_SLIPPAGE_BPS"),
		FeeBps:      envFloat("SIM_FEE_BPS"),
	}
}

func envFloat(key string) float64 {
	v := os.Getenv(key)
	if v == "" {
		return 0
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f < 0 || math.IsNaN(f) {
		return 0
	}
	return f
}

// adverseBps returns half-spread plus slippage for one side (buy adds, sell subtracts).
func (m Model) adverseBps() float64 {
	return m.SpreadBps/2 + m.SlippageBps
}

// EffectivePrice applies half-spread and slippage vs mid: buys pay more, sells receive less.
func (m Model) EffectivePrice(mid float64, side string) float64 {
	if mid <= 0 || math.IsNaN(mid) {
		return mid
	}
	adv := m.adverseBps() / 10_000
	switch side {
	case "Buy", "buy":
		return mid * (1 + adv)
	case "Sell", "sell":
		return mid * (1 - adv)
	default:
		return mid
	}
}

func (m Model) feeOnQuote(notionalQuote float64) float64 {
	if notionalQuote <= 0 || m.FeeBps <= 0 {
		return 0
	}
	return money.RoundQuote(notionalQuote * m.FeeBps / 10_000)
}

// BuyFill sizes a market buy spending up to quoteSpend quote at mid.
func (m Model) BuyFill(quoteSpend, mid float64) (baseQty, effPrice, fee, grossQuote float64, err error) {
	quoteSpend = money.RoundQuote(quoteSpend)
	if quoteSpend <= 0 || mid <= 0 {
		return 0, 0, 0, 0, errors.New("invalid buy")
	}
	effPrice = money.RoundQuote(m.EffectivePrice(mid, "Buy"))
	fee = m.feeOnQuote(quoteSpend)
	netQuote := money.RoundQuote(quoteSpend - fee)
	if netQuote <= 0 {
		return 0, 0, 0, 0, errors.New("quote below fee")
	}
	baseQty = money.RoundBase(netQuote / effPrice)
	grossQuote = quoteSpend
	return baseQty, effPrice, fee, grossQuote, nil
}

// SellFill sells baseAmt at mid; fee is on quote proceeds at effective price.
func (m Model) SellFill(baseAmt, mid float64) (quoteVal, effPrice, fee float64, err error) {
	baseAmt = money.RoundBase(baseAmt)
	if baseAmt <= 0 || mid <= 0 {
		return 0, 0, 0, errors.New("invalid sell")
	}
	effPrice = money.RoundQuote(m.EffectivePrice(mid, "Sell"))
	gross := money.RoundQuote(baseAmt * effPrice)
	fee = m.feeOnQuote(gross)
	quoteVal = money.RoundQuote(gross - fee)
	if quoteVal < 0 {
		quoteVal = 0
	}
	return quoteVal, effPrice, fee, nil
}
