package experimentapi

import (
	"strings"
	"testing"
	"time"
)

const testHour = int64(time.Hour / time.Millisecond)

func TestBackfillWindow(t *testing.T) {
	now := int64(1_700_000_000_000)
	now -= now % testHour // a closed hour boundary, so the open hour is now+0..1h
	cases := []struct {
		name             string
		from, to         int64
		wantFrom, wantTo int64
		wantErr          bool
	}{
		{"to clamped to now and rounded to the hour", now - 10*testHour, now + 5*testHour, now - 10*testHour, now, false},
		{"to rounded down inside the window", now - 10*testHour, now - 2*testHour + 1000, now - 10*testHour, now - 2*testHour, false},
		{"from zero rejected", 0, now, 0, 0, true},
		{"to before from rejected", now, now - testHour, 0, 0, true},
		{"range inside the open hour rejected", now, now + 30*60_000, 0, 0, true},
		{"range over 366 days rejected", now - 400*24*testHour, now, 0, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			from, to, err := BackfillWindow(c.from, c.to, now)
			if (err != nil) != c.wantErr {
				t.Fatalf("err %v, wantErr %v", err, c.wantErr)
			}
			if err == nil && (from != c.wantFrom || to != c.wantTo) {
				t.Fatalf("window [%d,%d) want [%d,%d)", from, to, c.wantFrom, c.wantTo)
			}
		})
	}
}

func TestBackfillMarkets(t *testing.T) {
	def, err := BackfillMarkets(nil)
	if err != nil || len(def) != 2 || def[0].Symbol != "BTCUSDT" || def[1].Symbol != "ETHUSDT" {
		t.Fatalf("default markets: %+v %v", def, err)
	}
	got, err := BackfillMarkets([]string{"USDCUSDT", "BTCUSDT", "USDCUSDT"})
	if err != nil || len(got) != 2 || got[0].Base != "USDC" || got[0].Quote != "USDT" || got[1].Symbol != "BTCUSDT" {
		t.Fatalf("dedup and order: %+v %v", got, err)
	}
	if _, err := BackfillMarkets([]string{"DOGEUSDT"}); err == nil {
		t.Fatal("unknown symbol must be rejected")
	}
}

func TestResolveBackfillProvider(t *testing.T) {
	if p, err := ResolveBackfillProvider(""); err != nil || p != "binance" {
		t.Fatalf("empty provider: %q %v", p, err)
	}
	if _, err := ResolveBackfillProvider("kraken"); err == nil {
		t.Fatal("unknown provider must be rejected")
	}
}

func TestBackfillInputForBaseURLPrecedence(t *testing.T) {
	m := BackfillMarket{Symbol: "BTCUSDT", Base: "BTC", Quote: "USDT"}
	cases := []struct {
		name  string
		saved map[string]string
		env   string
		want  string
	}{
		{"saved wins", map[string]string{"baseUrl": "https://mirror.example"}, "https://env.example", "https://mirror.example"},
		{"env when nothing saved", nil, "https://env.example", "https://env.example"},
		{"default when neither", map[string]string{}, "", "https://data-api.binance.vision"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := BackfillInputFor("binance", m, 1, 2, c.saved, c.env)
			if in.BinanceBaseURL != c.want || in.Base != "BTC" || in.Quote != "USDT" || in.Provider != "binance" || in.From != 1 || in.To != 2 {
				t.Fatalf("input %+v", in)
			}
		})
	}
}

func TestBackfillExecutionNameFitsStepFunctions(t *testing.T) {
	name := BackfillExecutionName("binance", "USDCUSDT", 1_700_000_000_000, 1_800_000_000_000, 1_900_000_000_000)
	if len(name) > 80 || !strings.HasPrefix(name, BackfillNamePrefix("binance", "USDCUSDT")) {
		t.Fatalf("name %q", name)
	}
}

func TestBackfillRunState(t *testing.T) {
	cases := []struct {
		status, cause, wantStatus, wantErr string
	}{
		{"RUNNING", "", "RUNNING", ""},
		{"SUCCEEDED", "", "SUCCEEDED", ""},
		{"FAILED", "IngestFailed", "FAILED", "IngestFailed"},
		{"FAILED", "", "FAILED", "workflow failed"},
		{"TIMED_OUT", "", "FAILED", "workflow timed_out"},
		{"ABORTED", "stopped by operator", "ABORTED", "stopped by operator"},
	}
	for _, c := range cases {
		st, msg := BackfillRunState(c.status, c.cause)
		if st != c.wantStatus || msg != c.wantErr {
			t.Errorf("%s/%q: got %s %q want %s %q", c.status, c.cause, st, msg, c.wantStatus, c.wantErr)
		}
	}
}
