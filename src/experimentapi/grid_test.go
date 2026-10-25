package experimentapi

import (
	"testing"

	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
)

func TestExpandGridMaCrossFilter(t *testing.T) {
	raw := expandGrid([]*v1.GridAxis{
		{Key: "fastHr", NumericValues: []float64{3, 6}},
		{Key: "slowHr", NumericValues: []float64{6, 12}},
	})
	if len(raw) != 4 {
		t.Fatalf("expected 4 raw combos, got %d", len(raw))
	}
	valid := filterValidConfigs(raw, "ma-cross")
	if len(valid) != 3 {
		t.Fatalf("expected 3 valid ma-cross combos, got %d", len(valid))
	}
}

func TestSignalConsensusFilter(t *testing.T) {
	raw := expandGrid([]*v1.GridAxis{
		{Key: "rsiBull", NumericValues: []float64{55.0}},
		{Key: "rsiBear", NumericValues: []float64{45.0}},
		{Key: "volRatioBull", NumericValues: []float64{1.2}},
		{Key: "volRatioBear", NumericValues: []float64{0.8}},
		{Key: "atrStopMult", NumericValues: []float64{2.5}},
		{Key: "minBarsFlatAfterSell", NumericValues: []float64{8}},
		{Key: "minBarsLongBeforeSell", NumericValues: []float64{4}},
	})
	valid := filterValidConfigs(raw, "signal-consensus")
	if len(valid) != 1 {
		t.Fatalf("expected 1 valid consensus combo, got %d", len(valid))
	}
}
