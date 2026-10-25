package calculation_signal_test

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strconv"
	"testing"
)

const sizedConfig = `STRATEGY_CONFIG={"fastHr":3,"slowHr":6,"sizing":{}}`

// kpiBodySized is a bar with both SMAs, the 24h ATR and a close.
func kpiBodySized(fast3h, slow6h, atr, closePrice float64) string {
	sma := func(hr int, v float64) map[string]any {
		return map[string]any{"def": map[string]any{"type": "MovingAverage", "method": "sma", "time_frame_hr": hr}, "value": v, "unit": "price"}
	}
	body := map[string]any{
		"close": closePrice,
		"results": []map[string]any{
			sma(3, fast3h), sma(6, slow6h),
			{"def": map[string]any{"type": "Atr", "method": "wilder", "time_frame_hr": 24}, "value": atr, "unit": "price"},
		},
	}
	b, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// seedBuyFill stores a sized buy signal (with its stop) and the matching journal fill.
func seedBuyFill(t *testing.T, dataDir string, closeMs int64, price, qty, stop float64) {
	t.Helper()
	sig, err := json.Marshal(map[string]any{"decision": "buy", "rule": "golden_cross", "stop_price": stop})
	if err != nil {
		t.Fatal(err)
	}
	sdb, err := sql.Open("sqlite", filepath.Join(dataDir, "calculation_signal.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer sdb.Close()
	if _, err := sdb.Exec(`CREATE TABLE IF NOT EXISTS signal (
  playbook TEXT NOT NULL, symbol TEXT NOT NULL, candle_close INTEGER NOT NULL, body TEXT NOT NULL,
  PRIMARY KEY (playbook, symbol, candle_close));`); err != nil {
		t.Fatal(err)
	}
	if _, err := sdb.Exec(`INSERT INTO signal VALUES (?, ?, ?, ?)`, playbookID, symbol, closeMs, string(sig)); err != nil {
		t.Fatal(err)
	}

	edb, err := sql.Open("sqlite", filepath.Join(dataDir, "execution.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer edb.Close()
	if _, err := edb.Exec(`CREATE TABLE IF NOT EXISTS fill_journal (
  order_link_id TEXT PRIMARY KEY, venue TEXT NOT NULL, venue_symbol TEXT NOT NULL, side TEXT NOT NULL,
  base_qty TEXT NOT NULL, quote_qty TEXT NOT NULL, price TEXT NOT NULL, fee TEXT NOT NULL,
  fee_currency TEXT NOT NULL, exchange_order_id TEXT NOT NULL, price_source TEXT NOT NULL);`); err != nil {
		t.Fatal(err)
	}
	link := playbookID + ":" + symbol + ":" + strconv.FormatInt(closeMs, 10)
	if _, err := edb.Exec(`INSERT INTO fill_journal VALUES (?, 'bybit', 'BTCUSDC', 'Buy', ?, ?, ?, '0', 'USDC', 'x', 'binance')`,
		link, fs(qty), fs(qty*price), fs(price)); err != nil {
		t.Fatal(err)
	}
}

func readRawSignal(t *testing.T, dataDir string, closeMs int64) map[string]any {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dataDir, "calculation_signal.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var raw string
	if err := db.QueryRow(`SELECT body FROM signal WHERE symbol = ? AND candle_close = ?`, symbol, closeMs).Scan(&raw); err != nil {
		t.Fatalf("read signal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSizedBuyCarriesStopAndNotional(t *testing.T) {
	requireDocker(t)
	dataDir := newDataDir(t)
	seedProcessSQLite(t, dataDir, []kpiSeed{
		{Symbol: symbol, BarOpen: barOpen() - threeHoursMs, Body: kpiBodySized(90, 100, 1000, 100000)},
		{Symbol: symbol, BarOpen: barOpen(), Body: kpiBodySized(110, 100, 1000, 100000)},
	})

	res := runContainerEnv(t, dataDir, symbol, candleClose, sizedConfig, "EQUITY=10000")
	assertRun(t, res, 0, 0)

	body := readRawSignal(t, dataDir, candleClose)
	if body["decision"] != "buy" || body["rule"] != "golden_cross" {
		t.Fatalf("got %v", body)
	}
	if stop, _ := body["stop_price"].(float64); stop != 98000 {
		t.Errorf("stop_price: got %v want 98000", body["stop_price"])
	}
	if n, _ := body["quote_notional"].(float64); n <= 0 || n > 10000 {
		t.Errorf("quote_notional: got %v", body["quote_notional"])
	}
}

func TestAtrStopExitsBeforeDeathCross(t *testing.T) {
	requireDocker(t)
	dataDir := newDataDir(t)
	// Fast stays above slow (no death cross) while the close falls through the stop.
	seedProcessSQLite(t, dataDir, []kpiSeed{
		{Symbol: symbol, BarOpen: barOpen() - threeHoursMs, Body: kpiBodySized(110, 100, 1000, 100000)},
		{Symbol: symbol, BarOpen: barOpen(), Body: kpiBodySized(110, 100, 1000, 97000)},
	})
	seedBuyFill(t, dataDir, candleClose-threeHoursMs, 100000, 0.05, 98000)

	res := runContainerEnv(t, dataDir, symbol, candleClose, sizedConfig, "EQUITY=10000")
	assertRun(t, res, 0, 0)

	body := readRawSignal(t, dataDir, candleClose)
	if body["decision"] != "sell" || body["rule"] != "atr_stop" {
		t.Fatalf("got %v", body)
	}
}

func TestNoSizingConfigLeavesSignalUnchanged(t *testing.T) {
	requireDocker(t)
	dataDir := newDataDir(t)
	seedProcessSQLite(t, dataDir, []kpiSeed{
		{Symbol: symbol, BarOpen: barOpen() - threeHoursMs, Body: kpiBodySized(90, 100, 1000, 100000)},
		{Symbol: symbol, BarOpen: barOpen(), Body: kpiBodySized(110, 100, 1000, 100000)},
	})

	// EQUITY alone must not turn sizing on.
	res := runContainerEnv(t, dataDir, symbol, candleClose, "EQUITY=10000")
	assertRun(t, res, 0, 0)

	body := readRawSignal(t, dataDir, candleClose)
	if body["decision"] != "buy" || body["rule"] != "golden_cross" {
		t.Fatalf("got %v", body)
	}
	for _, k := range []string{"stop_price", "quote_notional", "base_qty", "raise_stops_by"} {
		if _, ok := body[k]; ok {
			t.Errorf("unsized signal carries %s: %v", k, body)
		}
	}
}

func TestInvalidSizingFailsWithoutSignal(t *testing.T) {
	requireDocker(t)
	dataDir := newDataDir(t)
	seedProcessSQLite(t, dataDir, []kpiSeed{
		{Symbol: symbol, BarOpen: barOpen() - threeHoursMs, Body: kpiBodySized(90, 100, 1000, 100000)},
		{Symbol: symbol, BarOpen: barOpen(), Body: kpiBodySized(110, 100, 1000, 100000)},
	})

	res := runContainerEnv(t, dataDir, symbol, candleClose,
		`STRATEGY_CONFIG={"fastHr":3,"slowHr":6,"sizing":{"unitRiskPct":5}}`, "EQUITY=10000")
	assertRun(t, res, 1, 41)

	if rows := readSignals(t, dataDir); len(rows) != 0 {
		t.Fatalf("signal rows: got %d want 0", len(rows))
	}
}

func fs(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
