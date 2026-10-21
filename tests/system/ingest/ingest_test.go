package ingest

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
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
)

const image = "trading-copilot-ingest:test"

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
	if _, err := os.Stat(filepath.Join(dir, "parquet", "ETHUSDT.parquet")); !os.IsNotExist(err) {
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

type kline []any

type candle struct {
	OpenTime  int64  `parquet:"open_time"`
	Open      string `parquet:"open"`
	High      string `parquet:"high"`
	Low       string `parquet:"low"`
	Close     string `parquet:"close"`
	Volume    string `parquet:"volume"`
	CloseTime int64  `parquet:"close_time"`
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
	path := filepath.Join(dir, "parquet", symbol+".parquet")
	rows, err := parquet.ReadFile[candle](path)
	if err != nil {
		t.Fatalf("read parquet: %v", err)
	}
	return rows
}

func assertCandle(t *testing.T, got candle, open int64, o, h, l, c, vol string) {
	t.Helper()
	want := candle{
		OpenTime:  open,
		Open:      o,
		High:      h,
		Low:       l,
		Close:     c,
		Volume:    vol,
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
	rows, err := db.Query(`SELECT open_time FROM stored WHERE symbol = ? ORDER BY open_time`, symbol)
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
