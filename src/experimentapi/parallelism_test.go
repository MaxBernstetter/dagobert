package experimentapi

import (
	"strings"
	"testing"
)

func TestValidateParallelism(t *testing.T) {
	cases := []struct {
		name      string
		requested int
		max       int
		wantErr   string
	}{
		{"auto", 0, 22, ""},
		{"one", 1, 22, ""},
		{"at the maximum", 22, 22, ""},
		{"negative", -1, 22, "must be 0 (auto)"},
		{"above the maximum", 23, 22, "above the maximum 22"},
	}
	for _, c := range cases {
		err := validateParallelism(c.requested, c.max)
		switch {
		case c.wantErr == "" && err != nil:
			t.Errorf("%s: unexpected error %v", c.name, err)
		case c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)):
			t.Errorf("%s: error %v, want containing %q", c.name, err, c.wantErr)
		}
	}
}

func TestParallelismMaxFromEnv(t *testing.T) {
	t.Setenv("EXPERIMENT_MAX_PARALLELISM", "7")
	if got := parallelismMax(); got != 7 {
		t.Fatalf("env override: %d want 7", got)
	}
	t.Setenv("EXPERIMENT_MAX_PARALLELISM", "nope")
	if got := parallelismMax(); got < 1 {
		t.Fatalf("invalid env falls back to the CPU count, got %d", got)
	}
	t.Setenv("EXPERIMENT_MAX_PARALLELISM", "")
	if got := parallelismMax(); got < 1 {
		t.Fatalf("unset env falls back to the CPU count, got %d", got)
	}
}
