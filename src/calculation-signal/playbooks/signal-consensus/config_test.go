package signalconsensus

import (
	"testing"
)

func TestParseConfigDefaults(t *testing.T) {
	cfg, err := ParseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	def := DefaultConfig()
	if cfg != def {
		t.Fatalf("got %+v want %+v", cfg, def)
	}
}

func TestParseConfigRejectsUnknownKey(t *testing.T) {
	_, err := ParseConfig([]byte(`{"rsiBull":55,"extra":1}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateRsiOrder(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RsiBull = 40
	cfg.RsiBear = 50
	if err := cfg.Validate(0); err == nil {
		t.Fatal("expected rsi order error")
	}
}

func TestHashStable(t *testing.T) {
	cfg := DefaultConfig()
	h1, err := cfg.Hash()
	if err != nil {
		t.Fatal(err)
	}
	h2, err := cfg.Hash()
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 || h1 == "" {
		t.Fatalf("hash %q vs %q", h1, h2)
	}
}
