package ingest

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/parquet-go/parquet-go"
	_ "modernc.org/sqlite"

	"github.com/MaxBernstetter/dagobert/src/shared"
)

const image = "dagobert-ingest:test"

const hourMS int64 = 3_600_000

func TestMain(m *testing.M) {
	if _, err := exec.LookPath("docker"); err != nil {
		fmt.Fprintln(os.Stderr, "docker is unavailable: install Docker and ensure the docker CLI is on PATH")
		os.Exit(1)
	}
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "repo root: %v\n", err)
		os.Exit(1)
	}
	cmd := exec.Command("docker", "build", "-f", "docker/ingest/Dockerfile", "-t", image, ".")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "docker build failed: %v\n%s\n", err, out)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestTwoClosedHoursThenAlreadyStored(t *testing.T) {
	open0 := hourFloor(1_700_000_000_000)
	open1 := open0 + hourMS
	rows := []kline{
		candleRow(open0, "100.0", "110.0", "90.0", "105.0", "1.5"),
		candleRow(open1, "105.0", "120.0", "100.0", "115.0", "2.5"),
	}
	var hits atomic.Int32
	base := serve(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/api/v3/klines" {
			http.NotFound(w, r)
			return
		}
		writeJSON(t, w, rows)
	})

	dir := t.TempDir()

	stdout, stderr, code := runIngest(t, dir, base, nil, open0, open1+hourMS, "BTCUSDT")
	if code != 0 || !bytes.Contains(stdout, []byte("DETAIL=0\n")) {
		t.Fatalf("first run: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if hits.Load() < 1 {
		t.Fatal("fixture was not called")
	}
	got := readParquet(t, dir, "BTCUSDT")
	if len(got) != 2 {
		t.Fatalf("parquet rows: got %d want 2", len(got))
	}
	assertCandle(t, got[0], open0, "100.0", "110.0", "90.0", "105.0", "1.5")
	assertCandle(t, got[1], open1, "105.0", "120.0", "100.0", "115.0", "2.5")
	assertStored(t, dir, "BTCUSDT", []int64{open0, open1})

	before := hits.Load()
	stdout, stderr, code = runIngest(t, dir, base, nil, open0, open1+hourMS, "BTCUSDT")
	if code != 2 || !bytes.Contains(stdout, []byte("DETAIL=1\n")) {
		t.Fatalf("second run: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if hits.Load() != before {
		t.Fatalf("second run called fixture: before=%d after=%d", before, hits.Load())
	}
	if n := len(readParquet(t, dir, "BTCUSDT")); n != 2 {
		t.Fatalf("parquet row count after second run: %d", n)
	}
}

func TestStillOpenHour(t *testing.T) {
	now := time.Now().UTC()
	open := now.Truncate(time.Hour).UnixMilli()
	var hits atomic.Int32
	base := serve(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		writeJSON(t, w, []kline{})
	})
	dir := t.TempDir()

	stdout, stderr, code := runIngest(t, dir, base, nil, open, open+hourMS, "ETHUSDT")
	if code != 2 || !bytes.Contains(stdout, []byte("DETAIL=20\n")) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if hits.Load() != 0 {
		t.Fatalf("fixture calls: %d", hits.Load())
	}
	hiveETH, _ := shared.ParquetPath(dir, "binance", "ETH", "USDT", shared.YearUTC(open))
	if _, err := os.Stat(hiveETH); !os.IsNotExist(err) {
		t.Fatalf("parquet exists or stat error: %v", err)
	}
}

func TestMissingSymbol(t *testing.T) {
	base := serve(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("fixture should not be called")
	})
	dir := t.TempDir()
	stdout, stderr, code := runIngest(t, dir, base, nil, 1_700_000_000_000, 1_700_003_600_000, "")
	if code != 1 || !bytes.Contains(stdout, []byte("DETAIL=10\n")) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestRetryThenStore(t *testing.T) {
	open := hourFloor(1_600_000_000_000)
	var hits atomic.Int32
	base := serve(t, func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n == 1 {
			http.Error(w, "unavailable", http.StatusInternalServerError)
			return
		}
		writeJSON(t, w, []kline{candleRow(open, "1", "2", "0.5", "1.5", "3")})
	})
	dir := t.TempDir()
	env := map[string]string{
		"max_retries":    "3",
		"retry_wait":     "10ms",
		"max_retry_wait": "50ms",
		"backoff_factor": "1",
	}
	stdout, stderr, code := runIngest(t, dir, base, env, open, open+hourMS, "USDCUSDT")
	if code != 0 || !bytes.Contains(stdout, []byte("DETAIL=0\n")) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	got := readParquet(t, dir, "USDCUSDT")
	if len(got) != 1 {
		t.Fatalf("parquet rows: %d", len(got))
	}
	assertCandle(t, got[0], open, "1", "2", "0.5", "1.5", "3")
	assertStored(t, dir, "USDCUSDT", []int64{open})
}

func TestRetriesExhausted(t *testing.T) {
	open := hourFloor(1_500_000_000_000)
	base := serve(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusInternalServerError)
	})
	dir := t.TempDir()
	env := map[string]string{
		"max_retries":    "2",
		"retry_wait":     "10ms",
		"max_retry_wait": "20ms",
		"backoff_factor": "1",
	}
	stdout, stderr, code := runIngest(t, dir, base, env, open, open+hourMS, "BTCUSDT")
	if code != 1 || !bytes.Contains(stdout, []byte("DETAIL=30\n")) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	assertStored(t, dir, "BTCUSDT", nil)
}

func TestEmptyKlineMarksUnavailable(t *testing.T) {
	open := hourFloor(1_670_000_000_000)
	var hits atomic.Int32
	base := serve(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		writeJSON(t, w, []kline{})
	})
	dir := t.TempDir()
	stdout, stderr, code := runIngest(t, dir, base, nil, open, open+hourMS, "BTCUSDT")
	if code != 0 || !bytes.Contains(stdout, []byte("DETAIL=32\n")) {
		t.Fatalf("first run: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if hits.Load() < 1 {
		t.Fatal("fixture was not called")
	}
	assertStored(t, dir, "BTCUSDT", nil)
	assertUnavailable(t, dir, "BTCUSDT", []int64{open})
	hiveBTC, _ := shared.ParquetPath(dir, "binance", "BTC", "USDT", shared.YearUTC(open))
	if _, err := os.Stat(hiveBTC); !os.IsNotExist(err) {
		t.Fatalf("parquet should not exist: %v", err)
	}

	before := hits.Load()
	stdout, stderr, code = runIngest(t, dir, base, nil, open, open+hourMS, "BTCUSDT")
	if code != 2 || !bytes.Contains(stdout, []byte("DETAIL=1\n")) {
		t.Fatalf("second run: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if hits.Load() != before {
		t.Fatalf("second run called fixture: before=%d after=%d", before, hits.Load())
	}
}

func assertUnavailable(t *testing.T, dir, symbol string, want []int64) {
	t.Helper()
	path := filepath.Join(dir, "ingest.sqlite")
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs), RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	base, quote, ok := shared.SplitPipelineSymbol(symbol)
	if !ok {
		t.Fatalf("symbol %q", symbol)
	}
	rows, err := db.Query(
		`SELECT open_time FROM unavailable WHERE provider = ? AND base = ? AND quote = ? ORDER BY open_time`,
		"binance", base, quote,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []int64
	for rows.Next() {
		var ts int64
		if err := rows.Scan(&ts); err != nil {
			t.Fatal(err)
		}
		got = append(got, ts)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("unavailable keys: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unavailable keys: got %v want %v", got, want)
		}
	}
}

type kline []any

// candle is a stored hour with values as 8-decimal strings.
type candle struct {
	OpenTime  int64
	Open      string
	High      string
	Low       string
	Close     string
	Volume    string
	CloseTime int64
}

// diskCandle is the lake schema: DECIMAL(18,8) prices, DECIMAL(28,8) volume.
type diskCandle struct {
	OpenTime  int64    `parquet:"open_time"`
	Open      int64    `parquet:"open,decimal(8:18)"`
	High      int64    `parquet:"high,decimal(8:18)"`
	Low       int64    `parquet:"low,decimal(8:18)"`
	Close     int64    `parquet:"close,decimal(8:18)"`
	Volume    [12]byte `parquet:"volume,decimal(8:28)"`
	CloseTime int64    `parquet:"close_time"`
}

// legacyCandle is the old string schema, still readable.
type legacyCandle struct {
	OpenTime  int64  `parquet:"open_time"`
	Open      string `parquet:"open"`
	High      string `parquet:"high"`
	Low       string `parquet:"low"`
	Close     string `parquet:"close"`
	Volume    string `parquet:"volume"`
	CloseTime int64  `parquet:"close_time"`
}

func d8(u int64) string {
	sign := ""
	if u < 0 {
		sign, u = "-", -u
	}
	return fmt.Sprintf("%s%d.%08d", sign, u/100_000_000, u%100_000_000)
}

func vol8(b [12]byte) string {
	hi := new(big.Int).SetBytes(b[:])
	if b[0]&0x80 != 0 {
		hi.Sub(hi, new(big.Int).Lsh(big.NewInt(1), 96))
	}
	q, r := new(big.Int).QuoRem(hi, big.NewInt(100_000_000), new(big.Int))
	return fmt.Sprintf("%s.%08d", q, r.Int64())
}

func candleRow(open int64, o, h, l, c, vol string) kline {
	return kline{open, o, h, l, c, vol, open + hourMS - 1, "0", 1, "0", "0", "0"}
}

func hourFloor(ms int64) int64 {
	return ms - ms%hourMS
}

func serve(t *testing.T, h http.HandlerFunc) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: h}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	return "http://" + ln.Addr().String()
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Errorf("encode: %v", err)
	}
}

func runIngest(t *testing.T, dataDir, base string, env map[string]string, from, to int64, symbol string) (stdout, stderr []byte, code int) {
	t.Helper()
	args := []string{
		"run", "--rm", "--network", "host",
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"-e", "DATA_DIR=/data",
		"-e", "BINANCE_BASE_URL=" + base,
		"-v", dataDir + ":/data",
	}
	for k, v := range env {
		args = append(args, "-e", k+"="+v)
	}
	args = append(args, image, "-from", strconv.FormatInt(from, 10), "-to", strconv.FormatInt(to, 10))
	if symbol != "" {
		// legacy -symbol still accepted by ingest (split into base/quote)
		args = append(args, "-symbol", symbol)
	}
	cmd := exec.Command("docker", args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	code = 0
	if err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("docker run: %v stderr=%s", err, errBuf.String())
		}
		code = exit.ExitCode()
	}
	return outBuf.Bytes(), errBuf.Bytes(), code
}

func readParquet(t *testing.T, dir, symbol string) []candle {
	t.Helper()
	base, quote, ok := shared.SplitPipelineSymbol(symbol)
	if !ok {
		t.Fatalf("symbol %q", symbol)
	}
	paths, err := shared.ListCandleParquetPaths(dir, "binance", base, quote)
	if err != nil {
		t.Fatal(err)
	}
	var rows []candle
	for _, path := range paths {
		part, err := parquet.ReadFile[diskCandle](path)
		if err != nil {
			t.Fatalf("read parquet %s: %v", path, err)
		}
		for _, r := range part {
			rows = append(rows, candle{r.OpenTime, d8(r.Open), d8(r.High), d8(r.Low), d8(r.Close), vol8(r.Volume), r.CloseTime})
		}
	}
	return rows
}

func assertCandle(t *testing.T, got candle, open int64, o, h, l, c, vol string) {
	t.Helper()
	f8 := func(v string) string {
		x, err := strconv.ParseFloat(v, 64)
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("%.8f", x)
	}
	want := candle{
		OpenTime:  open,
		Open:      f8(o),
		High:      f8(h),
		Low:       f8(l),
		Close:     f8(c),
		Volume:    f8(vol),
		CloseTime: open + hourMS - 1,
	}
	if got != want {
		t.Fatalf("candle: got %+v want %+v", got, want)
	}
}

func assertStored(t *testing.T, dir, symbol string, want []int64) {
	t.Helper()
	path := filepath.Join(dir, "ingest.sqlite")
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs), RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	base, quote, ok := shared.SplitPipelineSymbol(symbol)
	if !ok {
		t.Fatalf("symbol %q", symbol)
	}
	rows, err := db.Query(
		`SELECT open_time FROM stored WHERE provider = ? AND base = ? AND quote = ? ORDER BY open_time`,
		"binance", base, quote,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []int64
	for rows.Next() {
		var ts int64
		if err := rows.Scan(&ts); err != nil {
			t.Fatal(err)
		}
		got = append(got, ts)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("stored keys: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("stored keys: got %v want %v", got, want)
		}
	}
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", wd)
		}
		dir = parent
	}
}

func TestTooManyDecimalsFailsLoudly(t *testing.T) {
	open := hourFloor(1_700_000_000_000)
	rows := []kline{candleRow(open, "100.0", "110.0", "90.0", "105.123456789", "1.5")}
	base := serve(t, func(w http.ResponseWriter, r *http.Request) { writeJSON(t, w, rows) })
	dir := t.TempDir()
	stdout, stderr, code := runIngest(t, dir, base, nil, open, open+hourMS, "BTCUSDT")
	if code != 1 || !bytes.Contains(stdout, []byte("DETAIL=50\n")) || !bytes.Contains(stderr, []byte("105.123456789")) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if n := len(readParquet(t, dir, "BTCUSDT")); n != 0 {
		t.Fatalf("rows stored despite bad value: %d", n)
	}
	assertStored(t, dir, "BTCUSDT", nil)
}

func TestPriceOverflowFailsLoudly(t *testing.T) {
	open := hourFloor(1_700_000_000_000)
	rows := []kline{candleRow(open, "100.0", "110.0", "90.0", "10000000000.0", "1.5")}
	base := serve(t, func(w http.ResponseWriter, r *http.Request) { writeJSON(t, w, rows) })
	stdout, stderr, code := runIngest(t, t.TempDir(), base, nil, open, open+hourMS, "BTCUSDT")
	if code != 1 || !bytes.Contains(stdout, []byte("DETAIL=50\n")) || !bytes.Contains(stderr, []byte("10000000000.0")) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestHugeVolumeAndLegacyFileAreKept(t *testing.T) {
	open0 := hourFloor(1_700_000_000_000)
	open1 := open0 + hourMS
	dir := t.TempDir()
	// Hour 0 already sits in a legacy string file; hour 1 arrives with a volume
	// above DECIMAL(18,8)'s 1e10 limit (a memecoin-sized hour).
	base0, quote0, _ := shared.SplitPipelineSymbol("SHIBUSDT")
	path, err := shared.ParquetPath(dir, "binance", base0, quote0, shared.YearUTC(open0))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := parquet.WriteFile(path, []legacyCandle{{open0, "0.00001000", "0.00001100", "0.00000900", "0.00001050", "1.50000000", open0 + hourMS - 1}}); err != nil {
		t.Fatal(err)
	}
	rows := []kline{candleRow(open1, "0.00001050", "0.00001200", "0.00001000", "0.00001100", "250000000000.12345678")}
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) { writeJSON(t, w, rows) })
	stdout, stderr, code := runIngest(t, dir, srv, nil, open0, open1+hourMS, "SHIBUSDT")
	if code != 0 || !bytes.Contains(stdout, []byte("DETAIL=0\n")) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	got := readParquet(t, dir, "SHIBUSDT")
	if len(got) != 2 {
		t.Fatalf("rows: %d", len(got))
	}
	assertCandle(t, got[0], open0, "0.00001", "0.000011", "0.000009", "0.0000105", "1.5")
	if got[1].Volume != "250000000000.12345678" || got[1].Close != "0.00001100" {
		t.Fatalf("second row: %+v", got[1])
	}
}
