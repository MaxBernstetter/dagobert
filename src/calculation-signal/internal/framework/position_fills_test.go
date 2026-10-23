package framework

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestDerivePositionFromFillsSim(t *testing.T) {
	t.Setenv("VENUE_SIMULATE", "1")
	dir := t.TempDir()

	dbPath := filepath.Join(dir, "execution.sqlite")
	execDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer execDB.Close()
	_, err = execDB.Exec(`CREATE TABLE fill_journal (
		order_link_id TEXT PRIMARY KEY,
		venue TEXT NOT NULL,
		venue_symbol TEXT NOT NULL,
		side TEXT NOT NULL,
		base_qty TEXT NOT NULL,
		quote_qty TEXT NOT NULL,
		price TEXT NOT NULL,
		fee TEXT NOT NULL,
		fee_currency TEXT NOT NULL,
		exchange_order_id TEXT NOT NULL,
		price_source TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatal(err)
	}
	insert := func(link, side string) {
		_, err := execDB.Exec(`INSERT INTO fill_journal (
			order_link_id, venue, venue_symbol, side, base_qty, quote_qty, price, fee, fee_currency, exchange_order_id, price_source
		) VALUES (?, 'bybit', 'BTCUSDC', ?, '0.01', '800', '80000', '0', 'USDC', 'sim', 'binance')`, link, side)
		if err != nil {
			t.Fatal(err)
		}
	}
	insert("ma-cross.yaml:BTCUSDT:2000", "Buy")

	sigDB, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer sigDB.Close()

	pos, err := DerivePosition(dir, sigDB, "ma-cross.yaml", "BTCUSDT", 3000)
	if err != nil || pos != "long" {
		t.Fatalf("got %q err=%v", pos, err)
	}

	insert("ma-cross.yaml:BTCUSDT:2500", "Sell")
	pos, err = DerivePosition(dir, sigDB, "ma-cross.yaml", "BTCUSDT", 3000)
	if err != nil || pos != "flat" {
		t.Fatalf("got %q err=%v", pos, err)
	}
}
