package framework

import (
	"testing"
)

func TestDerivePositionFromSignals(t *testing.T) {
	t.Setenv("VENUE_SIMULATE", "")
	dir := t.TempDir()
	db, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	insert := func(close int64, decision string) {
		body := `{"decision":"` + decision + `"}`
		_, err := db.Exec(
			`INSERT INTO signal (playbook, symbol, candle_close, body) VALUES (?, ?, ?, ?)`,
			"p.yaml", "BTCUSDT", close, body,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	insert(1000, "buy")
	insert(2000, "no-trade")
	pos, err := DerivePosition(t.TempDir(), db, "p.yaml", "BTCUSDT", 3000)
	if err != nil || pos != "long" {
		t.Fatalf("got %q err=%v", pos, err)
	}

	insert(3000, "sell")
	pos, err = DerivePosition(t.TempDir(), db, "p.yaml", "BTCUSDT", 4000)
	if err != nil || pos != "flat" {
		t.Fatalf("got %q err=%v", pos, err)
	}
}
