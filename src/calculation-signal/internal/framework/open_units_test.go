package framework

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestDeriveOpenUnitsFromJournal(t *testing.T) {
	dir := t.TempDir()
	edb, err := sql.Open("sqlite", filepath.Join(dir, "execution.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer edb.Close()
	if _, err := edb.Exec(`CREATE TABLE fill_journal (order_link_id TEXT PRIMARY KEY, venue TEXT, venue_symbol TEXT,
		side TEXT, base_qty TEXT, quote_qty TEXT, price TEXT, fee TEXT, fee_currency TEXT, exchange_order_id TEXT, price_source TEXT)`); err != nil {
		t.Fatal(err)
	}
	sdb, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer sdb.Close()

	fill := func(closeMs int64, side, qty, price, signal string) {
		t.Helper()
		link := "pb:BTCUSDT:" + string(rune('0'+closeMs))
		if _, err := edb.Exec(`INSERT INTO fill_journal VALUES (?, 'v', 's', ?, ?, '0', ?, '0', 'q', 'x', 'p')`, link, side, qty, price); err != nil {
			t.Fatal(err)
		}
		if err := CommitSignal(sdb, "pb", "BTCUSDT", closeMs, []byte(signal)); err != nil {
			t.Fatal(err)
		}
	}
	fill(1, "Buy", "1", "100", `{"decision":"buy","stop_price":90}`)
	fill(2, "Buy", "1", "110", `{"decision":"buy","stop_price":100,"raise_stops_by":5}`)
	fill(3, "Sell", "0.5", "120", `{"decision":"sell","base_qty":0.5}`)

	units, err := DeriveOpenUnits(dir, sdb, "pb", "BTCUSDT", 9, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 2 || units[0].Stop != 95 || units[1].Stop != 100 || units[0].Qty != 0.75 || units[1].Qty != 0.75 {
		t.Fatalf("units: %+v", units)
	}

	fill(4, "Sell", "1.5", "120", `{"decision":"sell"}`)
	if units, err = DeriveOpenUnits(dir, sdb, "pb", "BTCUSDT", 9, 1); err != nil || len(units) != 0 {
		t.Fatalf("after full exit: %+v err=%v", units, err)
	}
}
