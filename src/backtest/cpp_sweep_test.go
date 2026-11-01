package backtest

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	pb "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
)

func TestReplayActionsSizedNeedsEquity(t *testing.T) {
	n := 100.0
	rep := &pb.RunReport{Key: "k", Actions: []*pb.ExperimentInstanceAction{
		{BarOpenMs: 0, Side: "buy", QuoteNotional: &n},
	}}
	if _, err := ReplayActions(rep, 10); err == nil {
		t.Fatal("sized action without equity must fail")
	}
	eq := 1000.0
	rep.Actions[0].Equity = &eq
	got, err := ReplayActions(rep, 10)
	if err != nil || len(got) != 1 || got[0].CandleCloseMs != 10 || got[0].Sizing.Equity != 1000 {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestSweepPayloadRejectsOtherPlaybooks(t *testing.T) {
	if _, err := SweepPayload("position-sizing", nil); err == nil {
		t.Fatal("unsupported playbook must fail loudly")
	}
}

func TestSweepPayloadSignalConsensus(t *testing.T) {
	a, err := SweepPayload("signal-consensus", []byte(`{"rsiBull":55,"minBarsFlatAfterSell":3,"sizing":{"maxUnits":2}}`))
	if err != nil {
		t.Fatal(err)
	}
	var m pb.SignalConsensusConfig
	if err := a.UnmarshalTo(&m); err != nil {
		t.Fatal(err)
	}
	if m.RsiBull != 55 || m.RsiBear != 48 || m.AtrStopMult != 2.5 || m.MinBarsFlatAfterSell == nil || *m.MinBarsFlatAfterSell != 3 || m.MinBarsLongBeforeSell == nil || *m.MinBarsLongBeforeSell != 4 {
		t.Fatalf("resolved config not sent in full: %+v", &m)
	}
	if m.Sizing == nil || m.Sizing.MaxUnits == nil || *m.Sizing.MaxUnits != 2 || m.Sizing.UnitRiskPct == nil || *m.Sizing.UnitRiskPct != 1 {
		t.Fatalf("sizing: %+v", m.Sizing)
	}
	if _, err := SweepPayload("signal-consensus", nil); err != nil {
		t.Fatalf("defaults: %v", err)
	}
}

// An explicit 0 cooldown must reach C++ as a present field; absent keeps the default (#128).
func TestSweepPayloadSignalConsensusKeepsExplicitZeroCooldown(t *testing.T) {
	a, err := SweepPayload("signal-consensus", []byte(`{"minBarsFlatAfterSell":0,"minBarsLongBeforeSell":0}`))
	if err != nil {
		t.Fatal(err)
	}
	var m pb.SignalConsensusConfig
	if err := a.UnmarshalTo(&m); err != nil {
		t.Fatal(err)
	}
	if m.MinBarsFlatAfterSell == nil || *m.MinBarsFlatAfterSell != 0 || m.MinBarsLongBeforeSell == nil || *m.MinBarsLongBeforeSell != 0 {
		t.Fatalf("explicit 0 lost: %+v", &m)
	}
	if _, err := SweepPayload("signal-consensus", []byte(`{"rsiBull":40}`)); err == nil {
		t.Fatal("invalid config must fail Validate")
	}
}

func cppBinary(t *testing.T) string {
	t.Helper()
	return cppBinaryFor(t, "CPP_SMA_CROSS_BIN", "sma-cross", "sma-cross")
}

// cppBinaryFor resolves strategies/<dir>/strategy-<name> from env or the build presets, else skips.
func cppBinaryFor(t *testing.T, env, dir, name string) string {
	t.Helper()
	if p := os.Getenv(env); p != "" {
		return p
	}
	for _, preset := range []string{"optimized", "debug"} {
		p, _ := filepath.Abs(filepath.Join("..", "..", "build", preset, "strategies", dir, "strategy-"+name))
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skipf("strategy-%s not built (task cpp:build) and %s unset", name, env)
	return ""
}

// relEq is the equivalence tolerance of #130: relative with a tiny absolute floor.
func relEq(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol*math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
}

func compareJSON(t *testing.T, path string, a, b any, tol float64) {
	t.Helper()
	switch av := a.(type) {
	case map[string]any:
		bv, _ := b.(map[string]any)
		if len(av) != len(bv) {
			t.Errorf("%s: keys %d vs %d", path, len(av), len(bv))
		}
		for k, v := range av {
			compareJSON(t, path+"."+k, v, bv[k], tol)
		}
	case []any:
		bv, _ := b.([]any)
		if len(av) != len(bv) {
			t.Errorf("%s: len %d vs %d", path, len(av), len(bv))
			return
		}
		for i := range av {
			compareJSON(t, path+"[]", av[i], bv[i], tol)
		}
	case float64:
		bv, _ := b.(float64)
		if !relEq(av, bv, tol) {
			t.Errorf("%s: %v vs %v", path, av, bv)
		}
	default:
		if a != b {
			t.Errorf("%s: %v vs %v", path, a, b)
		}
	}
}
