package backtest

import (
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/parquet-go/parquet-go"
	_ "modernc.org/sqlite"

	"github.com/MaxBernstetter/dagobert/src/shared"
)

func TestCheckRawCoverageUnavailableReady(t *testing.T) {
	dir := t.TempDir()
	open0 := time.Date(2023, 3, 24, 12, 0, 0, 0, time.UTC).UnixMilli()
	open1 := open0 + hourMS
	open2 := open0 + 2*hourMS
	from := open0
	to := open2 + hourMS

	path, err := shared.ParquetPath(dir, "binance", "BTC", "USDT", shared.YearUTC(open0))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	writeHourParquet(t, path, []int64{open0, open2})
	writeUnavailable(t, dir, "binance", "BTC", "USDT", []int64{open1})

	cov, err := CheckRawCoverage(dir, "binance", "BTC", "USDT", from, to)
	if err != nil {
		t.Fatal(err)
	}
	if !cov.Ready {
		t.Fatalf("Ready=false missing=%v unavailable=%v", cov.MissingHours, cov.UnavailableHours)
	}
	if len(cov.MissingHours) != 0 {
		t.Fatalf("retryable missing: %v", cov.MissingHours)
	}
	if len(cov.UnavailableHours) != 1 || cov.UnavailableHours[0] != open1 {
		t.Fatalf("unavailable: %v", cov.UnavailableHours)
	}
	if cov.Have != 2 || cov.Expected != 3 {
		t.Fatalf("have=%d expected=%d", cov.Have, cov.Expected)
	}
}

func TestCheckRawCoverageRetryableNotReady(t *testing.T) {
	dir := t.TempDir()
	open0 := time.Date(2023, 3, 24, 12, 0, 0, 0, time.UTC).UnixMilli()
	open1 := open0 + hourMS
	from := open0
	to := open1 + hourMS
	path, err := shared.ParquetPath(dir, "binance", "BTC", "USDT", shared.YearUTC(open0))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	writeHourParquet(t, path, []int64{open0})

	cov, err := CheckRawCoverage(dir, "binance", "BTC", "USDT", from, to)
	if err != nil {
		t.Fatal(err)
	}
	if cov.Ready {
		t.Fatal("expected not ready")
	}
	if len(cov.MissingHours) != 1 || cov.MissingHours[0] != open1 {
		t.Fatalf("missing: %v", cov.MissingHours)
	}
}

type hourOnly struct {
	OpenTime  int64   `parquet:"open_time"`
	Open      float64 `parquet:"open"`
	High      float64 `parquet:"high"`
	Low       float64 `parquet:"low"`
	Close     float64 `parquet:"close"`
	Volume    float64 `parquet:"volume"`
	CloseTime int64   `parquet:"close_time"`
}

func writeHourParquet(t *testing.T, path string, opens []int64) {
	t.Helper()
	rows := make([]hourOnly, 0, len(opens))
	for _, ot := range opens {
		rows = append(rows, hourOnly{
			OpenTime: ot, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1,
			CloseTime: ot + hourMS - 1,
		})
	}
	if err := parquet.WriteFile(path, rows); err != nil {
		t.Fatal(err)
	}
}

func writeUnavailable(t *testing.T, dataDir, provider, base, quote string, opens []int64) {
	t.Helper()
	path := filepath.Join(dataDir, "ingest.sqlite")
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs), RawQuery: "_pragma=busy_timeout(5000)"}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS unavailable (
		provider TEXT NOT NULL,
		base TEXT NOT NULL,
		quote TEXT NOT NULL,
		open_time INTEGER NOT NULL,
		reason TEXT NOT NULL DEFAULT '',
		confirmed_at INTEGER NOT NULL,
		PRIMARY KEY (provider, base, quote, open_time)
	)`)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	for _, ot := range opens {
		if _, err := db.Exec(
			`INSERT INTO unavailable(provider, base, quote, open_time, reason, confirmed_at) VALUES (?, ?, ?, ?, ?, ?)`,
			provider, base, quote, ot, "binance_empty", now,
		); err != nil {
			t.Fatal(err)
		}
	}
}
