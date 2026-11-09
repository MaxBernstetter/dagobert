package process

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/parquet-go/parquet-go"

	"github.com/MaxBernstetter/dagobert/src/shared"
	"github.com/MaxBernstetter/dagobert/tests/system/internal/lakefile"
)

const image = "dagobert-calculation-technical-indicator"

const (
	hour = int64(3_600_000)
	tol  = 1e-9
)

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
	cmd := exec.Command("docker", "build", "-f", "docker/calculation-technical-indicator/Dockerfile", "-t", image, ".")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "docker build failed: %v\n%s\n", err, out)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

const threeHours = int64(10_800_000)

func TestThreeClosedHours(t *testing.T) {
	dir := dataDir(t)
	writeCandles(t, dir, "BTCUSDT", case1Candles())
	stdout, stderr, code := runProcess(t, dir, threeHours, "BTCUSDT", "0", strconv.FormatInt(threeHours, 10))
	if code != 0 || !hasDetail(stdout, 0) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	rows := readKPI(t, dir)
	if len(rows) != 1 {
		t.Fatalf("kpi rows: got %d want 1", len(rows))
	}
	assertBody(t, rows[0], case1Body())
}

func TestSameRangeAgain(t *testing.T) {
	dir := dataDir(t)
	writeCandles(t, dir, "BTCUSDT", case1Candles())
	args := []string{"BTCUSDT", "0", strconv.FormatInt(threeHours, 10)}
	stdout, stderr, code := runProcess(t, dir, threeHours, args...)
	if code != 0 || !hasDetail(stdout, 0) {
		t.Fatalf("first: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	first := readKPI(t, dir)
	if len(first) != 1 {
		t.Fatalf("kpi rows after first run: got %d want 1", len(first))
	}

	stdout, stderr, code = runProcess(t, dir, threeHours, args...)
	if code != 0 || !hasDetail(stdout, 0) {
		t.Fatalf("second: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	second := readKPI(t, dir)
	if len(second) != 1 || second[0] != first[0] {
		t.Fatalf("kpi rows changed:\nfirst  %v\nsecond %v", first, second)
	}
	assertBody(t, second[0], case1Body())
}

func TestStillOpen(t *testing.T) {
	dir := dataDir(t)
	writeCandles(t, dir, "BTCUSDT", case1Candles())
	stdout, stderr, code := runProcess(t, dir, threeHours-1, "BTCUSDT", "0", strconv.FormatInt(threeHours, 10))
	if code != 2 || !hasDetail(stdout, 21) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if n := len(readKPI(t, dir)); n != 0 {
		t.Fatalf("kpi rows: got %d want 0", n)
	}
}

func TestMissingCandle(t *testing.T) {
	t.Run("gap", func(t *testing.T) {
		dir := dataDir(t)
		all := case1Candles()
		writeCandles(t, dir, "BTCUSDT", all[:2])
		stdout, stderr, code := runProcess(t, dir, threeHours, "BTCUSDT", "0", strconv.FormatInt(threeHours, 10))
		if code != 2 || !hasDetail(stdout, 21) {
			t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
		if n := len(readKPI(t, dir)); n != 0 {
			t.Fatalf("kpi rows: got %d want 0", n)
		}
	})
	t.Run("absent", func(t *testing.T) {
		dir := dataDir(t)
		stdout, stderr, code := runProcess(t, dir, threeHours, "BTCUSDT", "0", strconv.FormatInt(threeHours, 10))
		if code != 2 || !hasDetail(stdout, 21) {
			t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
		if n := len(readKPI(t, dir)); n != 0 {
			t.Fatalf("kpi rows: got %d want 0", n)
		}
	})
}

func TestWrongArguments(t *testing.T) {
	for name, args := range map[string][]string{
		"symbol only":          {"BTCUSDT"},
		"single bar (removed)": {"BTCUSDT", "0"},
		"empty range":          {"BTCUSDT", "10800000", "10800000"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := dataDir(t)
			stdout, stderr, code := runProcess(t, dir, threeHours, args...)
			if code != 1 || !hasDetail(stdout, 10) {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if n := len(readKPI(t, dir)); n != 0 {
				t.Fatalf("kpi rows: got %d want 0", n)
			}
		})
	}
}

func TestTwoMovingAverageWindows(t *testing.T) {
	dir := dataDir(t)
	rows := append(case1Candles(),
		candle{10_800_000, 120, 140, 118, 130, 4, 14_399_999},
		candle{14_400_000, 130, 150, 125, 140, 5, 17_999_999},
		candle{18_000_000, 140, 160, 139, 150, 6, 21_599_999},
	)
	writeCandles(t, dir, "BTCUSDT", rows)
	stdout, stderr, code := runProcess(t, dir, 21_600_000, "BTCUSDT", "10800000", "21600000")
	if code != 0 || !hasDetail(stdout, 0) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	got := readKPI(t, dir)
	if len(got) != 1 {
		t.Fatalf("kpi rows: got %d want 1", len(got))
	}
	assertBody(t, got[0], wantBody{
		open: 120, high: 160, low: 118, close: 150, volume: 15,
		openTime: 10_800_000, closeTime: 21_599_999,
		results: []wantResult{
			{def: map[string]any{"type": "MovingAverage", "method": "sma", "time_frame_hr": 3}, value: 150, unit: "price"},
			{def: map[string]any{"type": "MovingAverage", "method": "sma", "time_frame_hr": 6}, value: 135, unit: "price"},
			{def: map[string]any{"type": "MovingAverage", "method": "ema", "time_frame_hr": 3}, value: 150, unit: "price"},
			{def: map[string]any{"type": "Rsi", "time_frame_hr": 3}, value: 100, unit: "index"},
			{def: map[string]any{"type": "Rsi", "time_frame_hr": 6}, value: 100, unit: "index"},
			{def: map[string]any{"type": "VolumeMean", "time_frame_hr": 3}, value: 15, unit: "amount"},
			{def: map[string]any{"type": "VolumeMean", "time_frame_hr": 6}, value: 10.5, unit: "amount"},
		},
	})
}

type candle struct {
	OpenTime  int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	CloseTime int64
}

func case1Candles() []candle {
	return []candle{
		{0, 100, 110, 90, 102, 1, 3_599_999},
		{hour, 102, 120, 101, 115, 2, 7_199_999},
		{2 * hour, 115, 130, 114, 120, 3, 10_799_999},
	}
}

type wantResult struct {
	def   map[string]any
	value float64
	unit  string
}

type wantBody struct {
	open, high, low, close, volume float64
	openTime, closeTime            int64
	results                        []wantResult
}

func case1Body() wantBody {
	return wantBody{
		open: 100, high: 130, low: 90, close: 120, volume: 6,
		openTime: 0, closeTime: 10_799_999,
		results: []wantResult{
			{def: map[string]any{"type": "MovingAverage", "method": "sma", "time_frame_hr": 3}, value: 120, unit: "price"},
			{def: map[string]any{"type": "MovingAverage", "method": "ema", "time_frame_hr": 3}, value: 120, unit: "price"},
			{def: map[string]any{"type": "Rsi", "time_frame_hr": 3}, value: 100, unit: "index"},
			{def: map[string]any{"type": "Rsi", "time_frame_hr": 6}, value: 100, unit: "index"},
			{def: map[string]any{"type": "VolumeMean", "time_frame_hr": 3}, value: 6, unit: "amount"},
		},
	}
}

func dataDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeCandles(t *testing.T, dir, symbol string, rows []candle) {
	t.Helper()
	if len(rows) == 0 {
		t.Fatal("no rows")
	}
	byYear := map[int][]candle{}
	for _, r := range rows {
		y := shared.YearUTC(r.OpenTime)
		byYear[y] = append(byYear[y], r)
	}
	for y, part := range byYear {
		path, err := shared.ParquetPath(dir, "binance", "BTC", "USDT", y)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := lakefile.Write(path, toLake(part)); err != nil {
			t.Fatalf("write parquet: %v", err)
		}
	}
}

func runProcess(t *testing.T, dataDir string, nowMS int64, args ...string) (stdout, stderr []byte, code int) {
	t.Helper()
	return runProcessWith(t, dataDir, nowMS, nil, args...)
}

// runProcessWith runs the job with extra environment. A non-empty env puts the container on the host
// network, so it reaches a Floci on a loopback port.
func runProcessWith(t *testing.T, dataDir string, nowMS int64, env map[string]string, args ...string) (stdout, stderr []byte, code int) {
	t.Helper()
	dockerArgs := []string{
		"run", "--rm",
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"-e", "DATA_DIR=/data",
		"-e", "NOW_MS=" + strconv.FormatInt(nowMS, 10),
		"-v", dataDir + ":/data",
	}
	if len(env) > 0 {
		dockerArgs = append(dockerArgs, "--network", "host")
	}
	for k, v := range env {
		dockerArgs = append(dockerArgs, "-e", k+"="+v)
	}
	dockerArgs = append(dockerArgs, image)
	dockerArgs = append(dockerArgs, args...)
	cmd := exec.Command("docker", dockerArgs...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("docker run: %v stderr=%s", err, errBuf.String())
		}
		code = exit.ExitCode()
	}
	return outBuf.Bytes(), errBuf.Bytes(), code
}

func hasDetail(stdout []byte, n int) bool {
	want := []byte("DETAIL=" + strconv.Itoa(n))
	for _, line := range bytes.Split(stdout, []byte("\n")) {
		if bytes.Equal(bytes.TrimSpace(line), want) {
			return true
		}
	}
	return false
}

// readKPI returns the stored indicator bars of the lake's TI cache (parquet/ti), ordered by bar open,
// each as the JSON body the consumers read: OHLCV, times and the indicator results.
func readKPI(t *testing.T, dir string) []string {
	t.Helper()
	type tiRow struct {
		BarOpen     int64   `parquet:"bar_open"`
		Open        float64 `parquet:"open"`
		High        float64 `parquet:"high"`
		Low         float64 `parquet:"low"`
		Close       float64 `parquet:"close"`
		Volume      float64 `parquet:"volume"`
		CloseTime   int64   `parquet:"close_time"`
		ResultsJSON string  `parquet:"results_json"`
	}
	var rows []tiRow
	root := filepath.Join(dir, "parquet", "ti")
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, "bars.parquet") {
			return nil
		}
		part, rerr := parquet.ReadFile[tiRow](p)
		if rerr != nil {
			t.Fatalf("read %s: %v", p, rerr)
		}
		rows = append(rows, part...)
		return nil
	})
	sort.Slice(rows, func(i, j int) bool { return rows[i].BarOpen < rows[j].BarOpen })
	var bodies []string
	for _, r := range rows {
		b, err := json.Marshal(map[string]any{
			"open": r.Open, "high": r.High, "low": r.Low, "close": r.Close, "volume": r.Volume,
			"open_time": r.BarOpen, "close_time": r.CloseTime, "results": json.RawMessage(r.ResultsJSON),
		})
		if err != nil {
			t.Fatal(err)
		}
		bodies = append(bodies, string(b))
	}
	return bodies
}

type storedBody struct {
	Open      float64        `json:"open"`
	High      float64        `json:"high"`
	Low       float64        `json:"low"`
	Close     float64        `json:"close"`
	Volume    float64        `json:"volume"`
	OpenTime  int64          `json:"open_time"`
	CloseTime int64          `json:"close_time"`
	Results   []storedResult `json:"results"`
}

type storedResult struct {
	Def   json.RawMessage `json:"def"`
	Value float64         `json:"value"`
	Unit  string          `json:"unit"`
}

func assertBody(t *testing.T, raw string, want wantBody) {
	t.Helper()
	var got storedBody
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("body json: %v\n%s", err, raw)
	}
	near(t, "open", got.Open, want.open)
	near(t, "high", got.High, want.high)
	near(t, "low", got.Low, want.low)
	near(t, "close", got.Close, want.close)
	near(t, "volume", got.Volume, want.volume)
	if got.OpenTime != want.openTime || got.CloseTime != want.closeTime {
		t.Fatalf("times: got open_time=%d close_time=%d want open_time=%d close_time=%d",
			got.OpenTime, got.CloseTime, want.openTime, want.closeTime)
	}
	if len(got.Results) != len(want.results) {
		t.Fatalf("results: got %d want %d\n%s", len(got.Results), len(want.results), raw)
	}
	used := make([]bool, len(got.Results))
	for _, w := range want.results {
		found := false
		for i, g := range got.Results {
			if used[i] || !defEqual(t, g.Def, w.def) {
				continue
			}
			used[i] = true
			found = true
			near(t, fmt.Sprintf("value %s", defCanon(w.def)), g.Value, w.value)
			if g.Unit != w.unit {
				t.Fatalf("unit for %s: got %q want %q", defCanon(w.def), g.Unit, w.unit)
			}
			break
		}
		if !found {
			t.Fatalf("missing result %s\n%s", defCanon(w.def), raw)
		}
	}
}

func defEqual(t *testing.T, raw json.RawMessage, want map[string]any) bool {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("def json: %v", err)
	}
	if len(got) != len(want) {
		return false
	}
	for k, wv := range want {
		gv, ok := got[k]
		if !ok || !scalarEqual(gv, wv) {
			return false
		}
	}
	return true
}

func scalarEqual(got, want any) bool {
	switch w := want.(type) {
	case string:
		g, ok := got.(string)
		return ok && g == w
	case int:
		g, ok := got.(float64)
		return ok && math.Abs(g-float64(w)) <= tol
	case float64:
		g, ok := got.(float64)
		return ok && math.Abs(g-w) <= tol
	default:
		return false
	}
}

func defCanon(fields map[string]any) string {
	b, err := json.Marshal(fields)
	if err != nil {
		return fmt.Sprint(fields)
	}
	return string(b)
}

func near(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Fatalf("%s: got %v want %v", name, got, want)
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

func toLake(rows []candle) []lakefile.Candle {
	out := make([]lakefile.Candle, len(rows))
	for i, r := range rows {
		out[i] = lakefile.Candle(r)
	}
	return out
}
