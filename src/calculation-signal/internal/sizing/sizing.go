// Package sizing holds the pure risk-sizing maths from docs/v2/features/position-sizing.md.
package sizing

import (
	"fmt"
	"math"
)

// Config is the optional `sizing` block of a strategy config. Zero fields mean default.
type Config struct {
	UnitRiskPct  float64 `json:"unitRiskPct"`
	HeatPct      float64 `json:"heatPct"`
	MaxUnits     int     `json:"maxUnits"`
	StopAtrMult  float64 `json:"stopAtrMult"`
	AddAtrMult   float64 `json:"addAtrMult"`
	MaxWeightPct float64 `json:"maxWeightPct"`
}

// Unit is one open buy: fill price, stop price and base quantity.
type Unit struct {
	Fill float64
	Stop float64
	Qty  float64
}

// WithDefaults fills zero fields with the documented defaults.
func (c Config) WithDefaults() Config {
	def := func(v *float64, d float64) {
		if *v == 0 {
			*v = d
		}
	}
	def(&c.UnitRiskPct, 1)
	def(&c.HeatPct, 2)
	def(&c.StopAtrMult, 2)
	def(&c.AddAtrMult, 0.5)
	def(&c.MaxWeightPct, 100)
	if c.MaxUnits == 0 {
		c.MaxUnits = 1
	}
	return c
}

// Validate checks the allowed ranges. Call on a config with defaults applied.
func (c Config) Validate() error {
	switch {
	case !(c.UnitRiskPct >= 0.25 && c.UnitRiskPct <= 2):
		return fmt.Errorf("sizing: unitRiskPct %v must be in [0.25, 2]", c.UnitRiskPct)
	case !(c.HeatPct >= c.UnitRiskPct && c.HeatPct <= 2):
		return fmt.Errorf("sizing: heatPct %v must be in [unitRiskPct, 2]", c.HeatPct)
	case c.MaxUnits < 1 || c.MaxUnits > 4:
		return fmt.Errorf("sizing: maxUnits %d must be in [1, 4]", c.MaxUnits)
	case !(c.StopAtrMult > 0):
		return fmt.Errorf("sizing: stopAtrMult %v must be positive", c.StopAtrMult)
	case !(c.AddAtrMult > 0):
		return fmt.Errorf("sizing: addAtrMult %v must be positive", c.AddAtrMult)
	case !(c.MaxWeightPct > 0 && c.MaxWeightPct <= 100):
		return fmt.Errorf("sizing: maxWeightPct %v must be in (0, 100]", c.MaxWeightPct)
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
	byRisk := equity * c.UnitRiskPct / 100 / (c.StopAtrMult * atr / fill)
	return math.Min(byRisk, equity*c.MaxWeightPct/100)
}

// StopPrice is the entry stop: fill minus stopAtrMult times ATR.
func StopPrice(fill, atr float64, c Config) float64 {
	return fill - c.StopAtrMult*atr
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
	dist := c.StopAtrMult * atr
	over := baseHeld*dist - equity*c.HeatPct/100
	if over <= 0 {
		return 0
	}
	return math.Min(over/dist, baseHeld)
}

// RaiseStops moves every earlier stop up by addAtrMult times ATR. A stop never falls.
func RaiseStops(units []Unit, atr float64, c Config) []Unit {
	out := make([]Unit, len(units))
	copy(out, units)
	if !valid(atr) {
		return out
	}
	step := c.AddAtrMult * atr
	for i := range out {
		out[i].Stop += step
	}
	return out
}

// PyramidAllowed reports whether another unit may be bought at price, and its notional.
// units are the open units (stops not yet raised); on true, callers raise them with RaiseStops.
func PyramidAllowed(units []Unit, lastFill, price, equity, atr float64, c Config) (float64, bool) {
	if len(units) == 0 || len(units) >= c.MaxUnits || !valid(lastFill, price, equity, atr) {
		return 0, false
	}
	if price-lastFill < c.AddAtrMult*atr {
		return 0, false
	}
	n := Notional(equity, atr, price, c)
	if n <= 0 {
		return 0, false
	}
	newRisk := n * c.StopAtrMult * atr / price
	total := OpenRisk(RaiseStops(units, atr, c)) + newRisk
	if total > equity*c.HeatPct/100*(1+1e-9) {
		return 0, false
	}
	return n, true
}
