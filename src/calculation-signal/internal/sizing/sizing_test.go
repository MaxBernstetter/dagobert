package sizing

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

type vecCfg = Config

type vectors struct {
	Notional []struct {
		Name              string
		Equity, Atr, Fill float64
		Cfg               vecCfg
		Want              float64
		WantRiskPct       float64
	}
	StopPrice []struct {
		Name            string
		Fill, Atr, Want float64
		Cfg             vecCfg
	}
	ExcessBase []struct {
		Name                        string
		BaseHeld, Atr, Equity, Want float64
		Cfg                         vecCfg
	}
	Pyramid []struct {
		Name                         string
		Cfg                          vecCfg
		Units                        []struct{ Fill, Stop, Qty float64 }
		LastFill, Price, Equity, Atr float64
		Want                         bool
	}
}

func near(a, b float64) bool { return math.Abs(a-b) <= 1e-6*math.Max(1, math.Abs(b)) }

func p[T any](v T) *T { return &v }

func TestVectors(t *testing.T) {
	raw, err := os.ReadFile("../../../../docs/v2/features/position-sizing-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var v vectors
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	for _, c := range v.Notional {
		cfg := c.Cfg.WithDefaults()
		got := Notional(c.Equity, c.Atr, c.Fill, cfg)
		if !near(got, c.Want) {
			t.Errorf("%s: notional %v want %v", c.Name, got, c.Want)
		}
		req, capped := RiskPct(got, c.Equity, c.Atr, c.Fill, cfg)
		if !near(req, cfg.unitRiskPct()) {
			t.Errorf("%s: requested risk %v want %v", c.Name, req, cfg.unitRiskPct())
		}
		if !near(capped, c.WantRiskPct) {
			t.Errorf("%s: capped risk %v want %v", c.Name, capped, c.WantRiskPct)
		}
	}
	for _, c := range v.StopPrice {
		if got := StopPrice(c.Fill, c.Atr, c.Cfg.WithDefaults()); !near(got, c.Want) {
			t.Errorf("%s: stop %v want %v", c.Name, got, c.Want)
		}
	}
	for _, c := range v.ExcessBase {
		if got := ExcessBaseForHeat(c.BaseHeld, c.Atr, c.Equity, c.Cfg.WithDefaults()); !near(got, c.Want) {
			t.Errorf("%s: excess %v want %v", c.Name, got, c.Want)
		}
	}
	for _, c := range v.Pyramid {
		var us []Unit
		for _, u := range c.Units {
			us = append(us, Unit(u))
		}
		if _, got := PyramidAllowed(us, c.LastFill, c.Price, c.Equity, c.Atr, c.Cfg.WithDefaults()); got != c.Want {
			t.Errorf("%s: pyramid %v want %v", c.Name, got, c.Want)
		}
	}
}

func TestValidate(t *testing.T) {
	if err := (Config{}).WithDefaults().Validate(); err != nil {
		t.Fatal(err)
	}
	for _, c := range []Config{
		{UnitRiskPct: p(3.0)},
		{UnitRiskPct: p(0.1)},
		{UnitRiskPct: p(1.5), HeatPct: p(1.0)},
		{HeatPct: p(3.0)},
		{MaxUnits: p(5)},
		{StopAtrMult: p(-1.0)},
		{MaxWeightPct: p(101.0)},
		{MaxWeightPct: p(0.0)},
		{UnitRiskPct: p(0.0)},
	} {
		if err := c.WithDefaults().Validate(); err == nil {
			t.Errorf("%+v: want error", c)
		}
	}
}

func TestExplicitZeroIsNotDefaulted(t *testing.T) {
	c := Config{MaxWeightPct: p(0.0)}.WithDefaults()
	if c.MaxWeightPct == nil || *c.MaxWeightPct != 0 {
		t.Fatalf("explicit 0 replaced: %+v", c.MaxWeightPct)
	}
}

func TestRaiseStopsNeverFall(t *testing.T) {
	out := RaiseStops([]Unit{{Fill: 100, Stop: 90, Qty: 1}}, 4, Config{}.WithDefaults())
	if out[0].Stop != 92 {
		t.Fatalf("stop %v", out[0].Stop)
	}
}

func TestRaiseStopsNegativeStepKeepsStops(t *testing.T) {
	out := RaiseStops([]Unit{{Fill: 100, Stop: 90, Qty: 1}}, 4, Config{AddAtrMult: p(-1.0)}.WithDefaults())
	if out[0].Stop != 90 {
		t.Fatalf("stop moved down to %v", out[0].Stop)
	}
}
