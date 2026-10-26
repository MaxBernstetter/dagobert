// Package sizing holds the pure risk-sizing maths from docs/v2/features/position-sizing.md.
package sizing

import (
	"fmt"
	"math"
)

// Documented defaults, applied to omitted (nil) fields only.
const (
	defaultUnitRiskPct   = 1
	defaultHeatPct       = 2
	defaultMaxUnits      = 1
	defaultStopAtrMult   = 2
	defaultAddAtrMult    = 0.5
	defaultMaxWeightPct  = 100
	defaultMinOrderQuote = 1.0
)

// Config is the optional `sizing` block of a strategy config. A nil field means the
// documented default; an explicit 0 is kept and fails Validate, except MinOrderQuote
// where 0 is valid and disables the strategy-level minimum.
type Config struct {
	UnitRiskPct  *float64 `json:"unitRiskPct,omitempty"`
	HeatPct      *float64 `json:"heatPct,omitempty"`
	MaxUnits     *int     `json:"maxUnits,omitempty"`
	StopAtrMult  *float64 `json:"stopAtrMult,omitempty"`
	AddAtrMult   *float64 `json:"addAtrMult,omitempty"`
	MaxWeightPct *float64 `json:"maxWeightPct,omitempty"`
	// MinOrderQuote is the smallest sized order in quote; smaller buys, pyramids and
	// vol_cut sells are skipped. Full exits are never skipped.
	MinOrderQuote *float64 `json:"minOrderQuote,omitempty"`
}

// Unit is one open buy: fill price, stop price and base quantity.
type Unit struct {
	Fill float64
	Stop float64
	Qty  float64
}

func fv(p *float64, d float64) float64 {
	if p == nil {
		return d
	}
	return *p
}

func (c Config) unitRiskPct() float64 { return fv(c.UnitRiskPct, defaultUnitRiskPct) }
func (c Config) heatPct() float64     { return fv(c.HeatPct, defaultHeatPct) }
func (c Config) stopAtrMult() float64 { return fv(c.StopAtrMult, defaultStopAtrMult) }
func (c Config) addAtrMult() float64  { return fv(c.AddAtrMult, defaultAddAtrMult) }
func (c Config) maxWeightPct() float64 {
	return fv(c.MaxWeightPct, defaultMaxWeightPct)
}

func (c Config) minOrderQuote() float64 { return fv(c.MinOrderQuote, defaultMinOrderQuote) }

// BelowMin reports whether a sized order of quote is under the minimum order and must be skipped.
func BelowMin(quote float64, c Config) bool { return quote < c.minOrderQuote() }

func (c Config) maxUnits() int {
	if c.MaxUnits == nil {
		return defaultMaxUnits
	}
	return *c.MaxUnits
}

// WithDefaults returns a copy with every nil field set to its documented default.
func (c Config) WithDefaults() Config {
	f := func(v *float64, d float64) *float64 {
		if v != nil {
			return v
		}
		return &d
	}
	c.UnitRiskPct = f(c.UnitRiskPct, defaultUnitRiskPct)
	c.HeatPct = f(c.HeatPct, defaultHeatPct)
	c.StopAtrMult = f(c.StopAtrMult, defaultStopAtrMult)
	c.AddAtrMult = f(c.AddAtrMult, defaultAddAtrMult)
	c.MaxWeightPct = f(c.MaxWeightPct, defaultMaxWeightPct)
	c.MinOrderQuote = f(c.MinOrderQuote, defaultMinOrderQuote)
	if c.MaxUnits == nil {
		n := defaultMaxUnits
		c.MaxUnits = &n
	}
	return c
}

// Validate checks the allowed ranges. Nil fields take their defaults first.
func (c Config) Validate() error {
	unit, heat, units := c.unitRiskPct(), c.heatPct(), c.maxUnits()
	stop, add, weight := c.stopAtrMult(), c.addAtrMult(), c.maxWeightPct()
	minOrder := c.minOrderQuote()
	switch {
	case !(minOrder >= 0):
		return fmt.Errorf("sizing: minOrderQuote %v must be >= 0", minOrder)
	case !(unit >= 0.25 && unit <= 2):
		return fmt.Errorf("sizing: unitRiskPct %v must be in [0.25, 2]", unit)
	case !(heat >= unit && heat <= 2):
		return fmt.Errorf("sizing: heatPct %v must be in [unitRiskPct, 2]", heat)
	case units < 1 || units > 4:
		return fmt.Errorf("sizing: maxUnits %d must be in [1, 4]", units)
	case !(stop > 0):
		return fmt.Errorf("sizing: stopAtrMult %v must be positive", stop)
	case !(add > 0):
		return fmt.Errorf("sizing: addAtrMult %v must be positive", add)
	case !(weight > 0 && weight <= 100):
		return fmt.Errorf("sizing: maxWeightPct %v must be in (0, 100]", weight)
	}
	return nil
}

func valid(vs ...float64) bool {
	for _, v := range vs {
		if !(v > 0) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

// Notional is the quote spend for one unit; 0 when an input is unusable.
func Notional(equity, atr, fill float64, c Config) float64 {
	if !valid(equity, atr, fill) {
		return 0
	}
	byRisk := equity * c.unitRiskPct() / 100 / (c.stopAtrMult() * atr / fill)
	return math.Min(byRisk, equity*c.maxWeightPct()/100)
}

// RiskPct returns the requested risk (unitRiskPct) and the risk percent of equity that
// notional actually carries at its stop, after the weight cap. Capped is 0 when an input
// is unusable.
func RiskPct(notional, equity, atr, fill float64, c Config) (requested, capped float64) {
	requested = c.unitRiskPct()
	if !valid(equity, atr, fill) || !valid(notional) {
		return requested, 0
	}
	capped = notional * (c.stopAtrMult() * atr / fill) / equity * 100
	return requested, capped
}

// StopPrice is the entry stop: fill minus stopAtrMult times ATR.
func StopPrice(fill, atr float64, c Config) float64 {
	return fill - c.stopAtrMult()*atr
}

// OpenRisk sums the entry-to-stop loss of open units, in quote.
func OpenRisk(units []Unit) float64 {
	var r float64
	for _, u := range units {
		r += math.Max(0, u.Qty*(u.Fill-u.Stop))
	}
	return r
}

// ExcessBaseForHeat is the base to sell so the held base, stopped at stopAtrMult
// times current ATR, risks at most heatPct of equity. The stop is unchanged.
func ExcessBaseForHeat(baseHeld, atr, equity float64, c Config) float64 {
	if !valid(baseHeld, atr, equity) {
		return 0
	}
	dist := c.stopAtrMult() * atr
	over := baseHeld*dist - equity*c.heatPct()/100
	if over <= 0 {
		return 0
	}
	return math.Min(over/dist, baseHeld)
}

// RaiseStops moves every earlier stop up by addAtrMult times ATR. A stop never falls,
// so a negative step is clamped to zero.
func RaiseStops(units []Unit, atr float64, c Config) []Unit {
	out := make([]Unit, len(units))
	copy(out, units)
	if !valid(atr) {
		return out
	}
	step := math.Max(0, c.addAtrMult()*atr)
	for i := range out {
		out[i].Stop += step
	}
	return out
}

// PyramidAllowed reports whether another unit may be bought at price, and its notional.
// units are the open units (stops not yet raised); on true, callers raise them with RaiseStops.
func PyramidAllowed(units []Unit, lastFill, price, equity, atr float64, c Config) (float64, bool) {
	if len(units) == 0 || len(units) >= c.maxUnits() || !valid(lastFill, price, equity, atr) {
		return 0, false
	}
	if price-lastFill < c.addAtrMult()*atr {
		return 0, false
	}
	n := Notional(equity, atr, price, c)
	if n <= 0 {
		return 0, false
	}
	newRisk := n * c.stopAtrMult() * atr / price
	total := OpenRisk(RaiseStops(units, atr, c)) + newRisk
	if total > equity*c.heatPct()/100*(1+1e-9) {
		return 0, false
	}
	return n, true
}
