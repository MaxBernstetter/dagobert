package backtest_test

import (
	"database/sql"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/MaxBernstetter/dagobert/src/backtest"
	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	"github.com/MaxBernstetter/dagobert/src/shared"
	"github.com/MaxBernstetter/dagobert/tests/system/internal/lakefile"
	_ "modernc.org/sqlite"
)

const (
	hour       = int64(3_600_000)
	threeHours = 3 * hour
	symbol     = "BTCUSDT"
)

type hourCandle = lakefile.Candle

func TestBacktestRunCorrectness(t *testing.T) {
	dataDir := tempDataDir(t)
	seedLiveMarker(t, dataDir)

	endMs := 72 * hour
	writeHourlyParquet(t, dataDir, symbol, endMs)

	cat, err := store.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer cat.Close()

	settings := store.DefaultSettings()
	settings.Base = "BTC"
	settings.Quote = "USDT"
	settings.Playbook = "ma-cross"
	settings.FromMs = 0
	settings.ToMs = endMs
	settings.RedownloadMissing = false
	settings.InitialUSDC = 50_000
	settings.InitialUSDT = 50_000

	rec, err := cat.CreateDraft("system test", "", settings)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("VENUE_SIMULATE", "1")
	if err := backtest.Run(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}

	got, err := cat.Get(rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != store.StatusCompleted {
		t.Fatalf("status %q error %q", got.Status, got.Error)
	}
	if got.ProgressTotal <= 0 || got.ProgressDone != got.ProgressTotal {
		t.Fatalf("progress %d/%d", got.ProgressDone, got.ProgressTotal)
	}

	if _, err := os.Stat(filepath.Join(dataDir, "technical_indicator.sqlite")); !os.IsNotExist(err) {
		t.Fatal("prod technical_indicator.sqlite must not be created by backtest")
	}
	if liveRows(t, dataDir) != 1 {
		t.Fatal("live calculation_signal.sqlite marker row was overwritten")
	}

	kpiRows := countBacktestKPI(t, dataDir, rec.ID)
	if kpiRows < 8 {
		t.Fatalf("backtest_kpi rows: got %d want at least 8", kpiRows)
	}

	ws := store.WorkspaceDir(dataDir, got)
	signals := countTable(t, filepath.Join(ws, "calculation_signal.sqlite"), "signal")
	if signals != got.ProgressTotal {
		t.Fatalf("signal rows %d != bars replayed %d", signals, got.ProgressTotal)
	}
	fills := countFills(t, ws)
	if fills < 1 {
		t.Fatalf("expected at least one simulated fill (ma-cross), got %d", fills)
	}
	if feeTotal(t, ws) <= 0 {
		t.Fatal("expected non-zero fees with default feeBps on backtest settings")
	}
	endCash := portfolioEndValue(t, ws)
	if math.IsNaN(endCash) || math.IsInf(endCash, 0) {
		t.Fatal("portfolio end value not finite")
	}
}

// Optional: DATA_DIR=./data go test ./tests/system/backtest -run TestSignalConsensusLakeChurn -v
func TestSignalConsensusLakeChurn(t *testing.T) {
	lake := os.Getenv("DATA_DIR")
	if lake == "" {
		t.Skip("DATA_DIR not set")
	}
	srcPaths, err := shared.ListCandleParquetPaths(lake, "binance", "BTC", "USDT")
	if err != nil {
		t.Fatal(err)
	}
	if len(srcPaths) == 0 {
		t.Skip("no lake parquet")
	}
	dataDir := tempDataDir(t)
	for _, src := range srcPaths {
		yearName := filepath.Base(filepath.Dir(src)) // year=YYYY
		destDir := filepath.Join(dataDir, "parquet", "provider=binance", "base=BTC", "quote=USDT", yearName)
		if err := os.MkdirAll(destDir, 0o777); err != nil {
			t.Fatal(err)
		}
		dest := filepath.Join(destDir, shared.CandlesParquetFile)
		if err := copyFile(src, dest); err != nil {
			t.Fatal(err)
		}
	}
	cat, err := store.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer cat.Close()
	settings := store.DefaultSettings()
	settings.Base = "BTC"
	settings.Quote = "USDT"
	settings.Playbook = "signal-consensus"
	settings.RedownloadMissing = false
	rec, err := cat.CreateDraft("churn-cap", "", settings)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("VENUE_SIMULATE", "1")
	if err := backtest.Run(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}
	ws := store.WorkspaceDir(dataDir, rec)
	fills := countFills(t, ws)
	buys := countSignalsWithDecision(t, ws, "buy")
	t.Logf("fills=%d buys=%d", fills, buys)
	const maxFills = 120
	if fills > maxFills {
		t.Fatalf("signal-consensus fills=%d exceeds cap %d (strategy too chatty)", fills, maxFills)
	}
}

func TestBacktestSignalConsensusEmitsFills(t *testing.T) {
	dataDir := tempDataDir(t)
	seedLiveMarker(t, dataDir)

	endMs := 4000 * hour
	writeTrendingHourlyParquet(t, dataDir, symbol, endMs)

	cat, err := store.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer cat.Close()

	settings := store.DefaultSettings()
	settings.Base = "BTC"
	settings.Quote = "USDT"
	settings.Playbook = "signal-consensus"
	settings.FromMs = 480 * hour
	settings.ToMs = endMs
	settings.RedownloadMissing = false

	rec, err := cat.CreateDraft("consensus system test", "", settings)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("VENUE_SIMULATE", "1")
	if err := backtest.Run(dataDir, rec.ID); err != nil {
		t.Fatal(err)
	}

	got, err := cat.Get(rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != store.StatusCompleted {
		t.Fatalf("status %q error %q", got.Status, got.Error)
	}
	ws := store.WorkspaceDir(dataDir, rec)
	buys := countSignalsWithDecision(t, ws, "buy")
	fills := countFills(t, ws)
	t.Logf("signal-consensus v10: buys=%d fills=%d", buys, fills)
	if buys >= 1 && fills < 1 {
		t.Fatalf("expected fill after buy, got fills=%d", fills)
	}
}

func TestTwoBacktestsDifferentKPIRanges(t *testing.T) {
	dataDir := tempDataDir(t)
	writeHourlyParquet(t, dataDir, symbol, 96*hour)

	cat, err := store.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer cat.Close()

	short := store.DefaultSettings()
	short.Base = "BTC"
	short.Quote = "USDT"
	short.FromMs = 0
	short.ToMs = 48 * hour
	short.RedownloadMissing = false

	long := short
	long.FromMs = 0
	long.ToMs = 96 * hour

	a, err := cat.CreateDraft("short", "", short)
	if err != nil {
		t.Fatal(err)
	}
	b, err := cat.CreateDraft("long", "", long)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("VENUE_SIMULATE", "1")
	if err := backtest.Run(dataDir, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := backtest.Run(dataDir, b.ID); err != nil {
		t.Fatal(err)
	}

	ka := countBacktestKPI(t, dataDir, a.ID)
	kb := countBacktestKPI(t, dataDir, b.ID)
	if ka <= 0 || kb <= ka {
		t.Fatalf("kpi counts short=%d long=%d; long range should have more rows", ka, kb)
	}
}

func copyFile(src, dst string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, in, 0o644)
}

func tempDataDir(t *testing.T) string {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := shared.EnsureDataLayout(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeTrendingHourlyParquet(t *testing.T, dataDir, sym string, endMs int64) {
	t.Helper()
	hours := endMs / hour
	if hours < 24 {
		t.Fatalf("need at least 24 hours of data, got %d", hours)
	}
	rows := make([]hourCandle, 0, hours)
	price := 100.0
	for i := 0; i < int(hours); i++ {
		switch {
		case i < int(hours)/5:
			price -= 0.2
		default:
			price += 2.5
		}
		ot := int64(i) * hour
		vol := 10.0
		if i >= int(hours)/2 {
			vol = 14.0
		}
		rows = append(rows, hourCandle{
			OpenTime: ot, Open: price, High: price + 1, Low: price - 1,
			Close: price, Volume: vol, CloseTime: ot + hour - 1,
		})
	}
	writeHourlyParquetRows(t, dataDir, sym, rows)
}

func writeHourlyParquet(t *testing.T, dataDir, sym string, endMs int64) {
	t.Helper()
	hours := endMs / hour
	if hours < 24 {
		t.Fatalf("need at least 24 hours of data, got %d", hours)
	}
	rows := make([]hourCandle, 0, hours)
	price := 100.0
	for i := 0; i < int(hours); i++ {
		if i < int(hours)/2 {
			price -= 0.4
		} else {
			price += 1.8
		}
		ot := int64(i) * hour
		vol := 10.0
		if i >= int(hours)/2 {
			vol = 14.0
		}
		rows = append(rows, hourCandle{
			OpenTime: ot, Open: price, High: price + 1, Low: price - 1,
			Close: price, Volume: vol, CloseTime: ot + hour - 1,
		})
	}
	writeHourlyParquetRows(t, dataDir, sym, rows)
}

func writeHourlyParquetRows(t *testing.T, dataDir, sym string, rows []hourCandle) {
	t.Helper()
	byYear := map[int][]hourCandle{}
	for _, r := range rows {
		y := shared.YearUTC(r.OpenTime)
		byYear[y] = append(byYear[y], r)
	}
	for y, part := range byYear {
		pqPath, err := shared.ParquetPath(dataDir, "binance", "BTC", "USDT", y)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(pqPath), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := lakefile.Write(pqPath, part); err != nil {
			t.Fatal(err)
		}
	}
}

func seedLiveMarker(t *testing.T, dataDir string) {
	t.Helper()
	dbPath := filepath.Join(dataDir, "calculation_signal.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`
CREATE TABLE signal (
  playbook TEXT NOT NULL,
  symbol TEXT NOT NULL,
  candle_close INTEGER NOT NULL,
  body TEXT NOT NULL,
  PRIMARY KEY (playbook, symbol, candle_close)
);
INSERT INTO signal (playbook, symbol, candle_close, body)
VALUES ('LIVE_MARKER', 'BTCUSDT', 1, '{}');
`)
	if err != nil {
		t.Fatal(err)
	}
}

func liveRows(t *testing.T, dataDir string) int {
	return countTable(t, filepath.Join(dataDir, "calculation_signal.sqlite"), "signal")
}

func countBacktestKPI(t *testing.T, dataDir, backtestID string) int {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dataDir, "backtests.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM backtest_kpi WHERE backtest_id = ?`, backtestID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func countTable(t *testing.T, dbPath, table string) int {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func countSignalsWithDecision(t *testing.T, workspace, decision string) int {
	t.Helper()
	dbPath := filepath.Join(workspace, "calculation_signal.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT body FROM signal`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var n int
	for rows.Next() {
		var body string
		if err := rows.Scan(&body); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(body, `"decision":"`+decision+`"`) {
			n++
		}
	}
	return n
}

func countFills(t *testing.T, workspace string) int {
	t.Helper()
	dbPath := filepath.Join(workspace, "execution.sqlite")
	if _, err := os.Stat(dbPath); err != nil {
		return 0
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	err = db.QueryRow(`SELECT COUNT(*) FROM fill_journal`).Scan(&n)
	if err != nil {
		err = db.QueryRow(`SELECT COUNT(*) FROM order_result WHERE kind = 'fill'`).Scan(&n)
	}
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func feeTotal(t *testing.T, workspace string) float64 {
	t.Helper()
	dbPath := filepath.Join(workspace, "execution.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var sum float64
	rows, err := db.Query(`SELECT fee FROM fill_journal`)
	if err != nil {
		return 0
	}
	defer rows.Close()
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			t.Fatal(err)
		}
		v, _ := strconv.ParseFloat(f, 64)
		sum += v
	}
	return sum
}

func portfolioEndValue(t *testing.T, workspace string) float64 {
	t.Helper()
	dbPath := filepath.Join(workspace, "execution.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var cash float64
	_ = db.QueryRow(`SELECT SUM(CAST(cash_balance AS REAL)) FROM balance`).Scan(&cash)
	return cash
}
