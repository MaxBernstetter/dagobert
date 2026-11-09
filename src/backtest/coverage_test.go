package backtest

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MaxBernstetter/dagobert/src/internal/candlefile"
	"github.com/MaxBernstetter/dagobert/src/internal/ingestledger"
	"github.com/MaxBernstetter/dagobert/src/shared"
	"github.com/MaxBernstetter/dagobert/src/shared/pgtest"
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

type hourOnly = candlefile.FloatRow

func writeHourParquet(t *testing.T, path string, opens []int64) {
	t.Helper()
	rows := make([]hourOnly, 0, len(opens))
	for _, ot := range opens {
		rows = append(rows, hourOnly{
			OpenTime: ot, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1,
			CloseTime: ot + hourMS - 1,
		})
	}
	if err := candlefile.WriteFloatFile(path, rows); err != nil {
		t.Fatal(err)
	}
}

// writeUnavailable records confirmed-empty hours in a scratch ledger and points DATABASE_URL at it.
func writeUnavailable(t *testing.T, _ string, provider, base, quote string, opens []int64) {
	t.Helper()
	dbURL := pgtest.ScratchDB(t)
	t.Setenv("DATABASE_URL", dbURL)
	l, err := ingestledger.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err := l.MarkUnavailable(context.Background(), provider, base, quote, opens, "binance_empty", time.Now().UnixMilli()); err != nil {
		t.Fatal(err)
	}
}
