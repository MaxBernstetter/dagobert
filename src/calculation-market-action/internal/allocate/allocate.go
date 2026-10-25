package allocate

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/MaxBernstetter/dagobert/src/internal/money"
)

// Policy controls layer-2 sizing from venue balances.
type Policy struct {
	TargetWeightPct float64 // max portfolio weight in the asset (0–100)
	ReserveQuote    float64 // USDC left untouchable
	MinOrderQuote   float64 // venue minimum spend (USDC)
}

func DefaultPolicy() Policy {
	return Policy{
		TargetWeightPct: 10,
		ReserveQuote:    50,
		MinOrderQuote:   1,
	}
}

// Input is live venue cash + base and a mark price in quote per base.
type Input struct {
	SignalDecision string
	QuoteCash      float64
	BaseHeld       float64
	MarkPrice      float64
}

// Result is the sized action or a no-trade with reason.
type Result struct {
	Decision string
	Size     *float64
	Rule     string
	Note     string
}

func Size(in Input, p Policy) Result {
	switch in.SignalDecision {
	case "no-trade":
		return Result{Decision: "no-trade", Rule: "none"}
	case "buy":
		return sizeBuy(in, p)
	case "sell":
		return sizeSell(in, p)
	default:
		return Result{Decision: "no-trade", Rule: "none", Note: "unknown_signal"}
	}
}

func sizeBuy(in Input, p Policy) Result {
	if in.MarkPrice <= 0 || math.IsNaN(in.MarkPrice) {
		return Result{Decision: "no-trade", Rule: "none", Note: "no_mark_price"}
	}
	portfolio := in.QuoteCash + in.BaseHeld*in.MarkPrice
	if portfolio <= 0 {
		return Result{Decision: "no-trade", Rule: "none", Note: "empty_portfolio"}
	}
	current := in.BaseHeld * in.MarkPrice
	targetValue := portfolio * (p.TargetWeightPct / 100)
	room := targetValue - current
	if room <= 0 {
		return Result{Decision: "no-trade", Rule: "none", Note: "at_or_above_target_weight"}
	}
	spendable := in.QuoteCash - p.ReserveQuote
	if spendable <= 0 {
		return Result{Decision: "no-trade", Rule: "none", Note: "insufficient_cash_after_reserve"}
	}
	quote := math.Min(room, spendable)
	if quote < p.MinOrderQuote {
		return Result{Decision: "no-trade", Rule: "none", Note: "below_min_order"}
	}
	q := money.RoundQuote(quote)
	return Result{Decision: "buy", Size: &q, Rule: "allocate_buy"}
}

func sizeSell(in Input, p Policy) Result {
	if in.BaseHeld <= 0 {
		return Result{Decision: "no-trade", Rule: "none", Note: "no_base_to_sell"}
	}
	b := money.FloorBase(in.BaseHeld)
	if b <= 0 {
		return Result{Decision: "no-trade", Rule: "none", Note: "dust_position"}
	}
	return Result{Decision: "sell", Size: &b, Rule: "allocate_sell"}
}

func ParseBalance(s string) (float64, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(f) {
		return 0, fmt.Errorf("balance %q", s)
	}
	return f, nil
}
