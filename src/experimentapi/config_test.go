package experimentapi

import (
	"testing"

	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
)

func consensusSpec(flat, long *int32) *v1.ExperimentInstanceSpec {
	return &v1.ExperimentInstanceSpec{SignalConsensus: &v1.SignalConsensusConfig{
		RsiBull: 52, RsiBear: 48, VolRatioBull: 1.1, VolRatioBear: 0.85, AtrStopMult: 2.5,
		MinBarsFlatAfterSell: flat, MinBarsLongBeforeSell: long,
	}}
}

func i32(v int32) *int32 { return &v }

func cooldowns(t *testing.T, spec *v1.ExperimentInstanceSpec) (int32, int32) {
	t.Helper()
	cfg, err := normalizeSignalConsensusConfig(strategyMapFromSpec(spec))
	if err != nil {
		t.Fatal(err)
	}
	return cfg.GetMinBarsFlatAfterSell(), cfg.GetMinBarsLongBeforeSell()
}

func TestCooldownPointersKeepUnsetDefaultAndExplicitZero(t *testing.T) {
	if f, l := cooldowns(t, consensusSpec(nil, nil)); f != 8 || l != 4 {
		t.Fatalf("unset cooldowns: %d/%d want default 8/4", f, l)
	}
	if f, l := cooldowns(t, consensusSpec(i32(0), i32(0))); f != 0 || l != 0 {
		t.Fatalf("explicit 0 cooldowns: %d/%d want 0/0", f, l)
	}
	if f, l := cooldowns(t, consensusSpec(i32(3), nil)); f != 3 || l != 4 {
		t.Fatalf("flat 3, long unset: %d/%d want 3/4", f, l)
	}
}

func TestCooldownsChangeTheConfigHash(t *testing.T) {
	hashOf := func(spec *v1.ExperimentInstanceSpec) string {
		t.Helper()
		n, err := normalizeInstanceConfig("signal-consensus", strategyMapFromSpec(spec))
		if err != nil {
			t.Fatal(err)
		}
		return n.Hash
	}
	unset := hashOf(consensusSpec(nil, nil))
	zero := hashOf(consensusSpec(i32(0), nil))
	three := hashOf(consensusSpec(i32(3), nil))
	if unset == zero || zero == three || unset == three {
		t.Fatalf("cooldown variants must hash apart: unset=%s zero=%s three=%s", unset, zero, three)
	}
	if hashOf(consensusSpec(i32(8), i32(4))) != unset {
		t.Fatal("explicit defaults must hash like unset cooldowns")
	}
}

// The grid path: the axes become specs, and each spec is hashed as the create path does.
func TestGridCooldownAxesGiveDistinctConfigs(t *testing.T) {
	specs, err := specsFromGrid("signal-consensus", []*v1.GridAxis{
		{Key: "rsiBull", NumericValues: []float64{52}},
		{Key: "rsiBear", NumericValues: []float64{48}},
		{Key: "volRatioBull", NumericValues: []float64{1.1}},
		{Key: "volRatioBear", NumericValues: []float64{0.85}},
		{Key: "atrStopMult", NumericValues: []float64{2.5}},
		{Key: "minBarsFlatAfterSell", NumericValues: []float64{0, 3, 8}},
		{Key: "minBarsLongBeforeSell", NumericValues: []float64{0, 4}},
	}, "BTCUSDT")
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 6 {
		t.Fatalf("specs %d want 6 (3 flat x 2 long)", len(specs))
	}
	seen := map[string]bool{}
	for _, spec := range specs {
		n, err := normalizeInstanceConfig("signal-consensus", strategyMapFromSpec(spec))
		if err != nil {
			t.Fatal(err)
		}
		if seen[n.Hash] {
			t.Fatalf("duplicate config hash %s: cooldowns collapsed", n.Hash)
		}
		seen[n.Hash] = true
	}
}

func TestNumberValReadsSetPointersOnly(t *testing.T) {
	if v, ok := numberVal(i32(0)); !ok || v != 0 {
		t.Fatalf("set *int32 0: %v %v", v, ok)
	}
	var nilInt *int32
	if _, ok := numberVal(nilInt); ok {
		t.Fatal("nil *int32 must read as not set")
	}
	f := 2.5
	if v, ok := numberVal(&f); !ok || v != 2.5 {
		t.Fatalf("set *float64: %v %v", v, ok)
	}
}
