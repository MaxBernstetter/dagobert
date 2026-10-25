package macross

import (
	"encoding/json"
	"testing"
)

func TestParseConfigDefaults(t *testing.T) {
	cfg, err := ParseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg != DefaultConfig() {
		t.Fatalf("got %+v", cfg)
	}
}

func TestParseConfigRejectsUnknownKey(t *testing.T) {
	_, err := ParseConfig([]byte(`{"fastHr":3,"slowHr":6,"extra":1}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateOrdering(t *testing.T) {
	cfg := Config{FastHr: 6, SlowHr: 3}
	if err := cfg.Validate(3 * 60 * 60 * 1000); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateDeployedAndDivisible(t *testing.T) {
	cfg := Config{FastHr: 3, SlowHr: 6}
	if err := cfg.Validate(3 * 60 * 60 * 1000); err != nil {
		t.Fatal(err)
	}
	cfg.FastHr = 5
	if err := cfg.Validate(3 * 60 * 60 * 1000); err == nil {
		t.Fatal("expected undeployed hour error")
	}
}

func TestEvaluateUsesConfigWindows(t *testing.T) {
	s := Strategy{}
	raw, _ := json.Marshal(Config{FastHr: 6, SlowHr: 12})
	ctx := frameworkContext(raw, "flat", true, dualSMAAt(6, 12, 110, 100), dualSMAAt(6, 12, 90, 100))
	dec, err := s.Evaluate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Decision != "buy" || dec.RuleID != "golden_cross" {
		t.Fatalf("got %+v", dec)
	}
}

func TestHashStable(t *testing.T) {
	a, err := Config{FastHr: 3, SlowHr: 6}.Hash()
	if err != nil {
		t.Fatal(err)
	}
	b, err := Config{FastHr: 3, SlowHr: 6}.Hash()
	if err != nil {
		t.Fatal(err)
	}
	if a != b || a == "" {
		t.Fatalf("hashes %q %q", a, b)
	}
	c, err := Config{FastHr: 6, SlowHr: 12}.Hash()
	if err != nil {
		t.Fatal(err)
	}
	if a == c {
		t.Fatal("expected distinct hashes")
	}
}
