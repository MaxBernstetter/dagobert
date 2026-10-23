package calculation_signal_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

const (
	imageTag     = "trading-copilot-calculation-signal:test"
	candleClose  = int64(1700000000000)
	threeHoursMs = int64(3 * 60 * 60 * 1000)
	symbol       = "BTCUSDT"
	playbookID   = "ma-cross.yaml"
)

var (
	dockerOK bool
	repoRoot string
	detailRe = regexp.MustCompile(`(?m)^DETAIL=(\d+)\s*$`)
)

func TestMain(m *testing.M) {
	root, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "find repo root: %v\n", err)
		os.Exit(1)
	}
	repoRoot = root

	if exec.Command("docker", "version").Run() == nil {
		build := exec.Command(
			"docker", "build",
			"-f", "docker/calculation-signal/Dockerfile",
			"-t", imageTag,
			".",
		)
		build.Dir = repoRoot
		build.Stdout = os.Stdout
		build.Stderr = os.Stderr
		if err := build.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "docker build failed: %v\n", err)
			os.Exit(1)
		}
		dockerOK = true
	}

	os.Exit(m.Run())
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "docker", "calculation-signal", "Dockerfile")); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("trading-copilot repo root not found from %s", dir)
		}
		dir = parent
	}
}

func requireDocker(t *testing.T) {
	t.Helper()
	if !dockerOK {
		t.Skip("docker not available")
	}
}

func barOpen() int64 {
	return candleClose - threeHoursMs
}

type actionRow struct {
	Playbook    string
	Symbol      string
	CandleClose int64
	Body        actionBody
}

type actionBody struct {
	Decision  string          `json:"decision"`
	Size      *float64        `json:"size"`
	Exit      json.RawMessage `json:"exit"`
	Rule      string          `json:"rule"`
	WrittenAt string          `json:"written_at,omitempty"`
}

type runResult struct {
	ExitCode int
	Detail   int
	Stdout   string
}

func runContainer(t *testing.T, dataDir, sym string, closeMs int64) runResult {
	t.Helper()
	args := []string{
		"run", "--rm",
		"-e", "DATA_DIR=/data",
		"-e", "PLAYBOOK=ma-cross",
		"-v", dataDir + ":/data",
		imageTag,
		sym, fmt.Sprintf("%d", closeMs),
	}
	cmd := exec.Command("docker", args...)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	stdout := string(out)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return runResult{
				ExitCode: exitErr.ExitCode(),
				Detail:   parseDetail(stdout),
				Stdout:   stdout,
			}
		}
		t.Fatalf("docker run: %v\noutput:\n%s", err, stdout)
	}
	return runResult{ExitCode: 0, Detail: parseDetail(stdout), Stdout: stdout}
}

func parseDetail(output string) int {
	m := detailRe.FindStringSubmatch(output)
	if len(m) != 2 {
		return -1
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

func newDataDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	return dir
}

func seedProcessSQLite(t *testing.T, dir string, rows []kpiSeed) {
	t.Helper()
	path := filepath.Join(dir, "technical_indicator.sqlite")
	if err := os.MkdirAll(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS kpi (
  symbol TEXT NOT NULL,
  bar_open INTEGER NOT NULL,
  body TEXT NOT NULL,
  PRIMARY KEY (symbol, bar_open)
);`); err != nil {
		t.Fatal(err)
	}

	for _, row := range rows {
		_, err := db.Exec(
			`INSERT INTO kpi (symbol, bar_open, body) VALUES (?, ?, ?)`,
			row.Symbol, row.BarOpen, row.Body,
		)
		if err != nil {
			t.Fatal(err)
		}
	}
}

type kpiSeed struct {
	Symbol  string
	BarOpen int64
	Body    string
}

func kpiBodyDualSMA(fast3h, slow6h float64) string {
	body := map[string]any{
		"results": []map[string]any{
			{
				"def": map[string]any{
					"type": "MovingAverage", "method": "sma", "time_frame_hr": 3,
				},
				"value": fast3h,
				"unit":  "price",
			},
			{
				"def": map[string]any{
					"type": "MovingAverage", "method": "sma", "time_frame_hr": 6,
				},
				"value": slow6h,
				"unit":  "price",
			},
		},
	}
	b, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func seedSignal(t *testing.T, dataDir, symbol string, closeMs int64, decision, rule string) {
	t.Helper()
	path := filepath.Join(dataDir, "calculation_signal.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS signal (
  playbook TEXT NOT NULL,
  symbol TEXT NOT NULL,
  candle_close INTEGER NOT NULL,
  body TEXT NOT NULL,
  PRIMARY KEY (playbook, symbol, candle_close)
);`); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]string{"decision": decision, "rule": rule})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		`INSERT INTO signal (playbook, symbol, candle_close, body) VALUES (?, ?, ?, ?)`,
		playbookID, symbol, closeMs, string(body),
	)
	if err != nil {
		t.Fatal(err)
	}
}

func readSignals(t *testing.T, dataDir string) []actionRow {
	t.Helper()
	path := filepath.Join(dataDir, "calculation_signal.sqlite")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT playbook, symbol, candle_close, body FROM signal ORDER BY playbook, symbol, candle_close`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var out []actionRow
	for rows.Next() {
		var r actionRow
		var bodyJSON string
		if err := rows.Scan(&r.Playbook, &r.Symbol, &r.CandleClose, &bodyJSON); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(bodyJSON), &r.Body); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func assertRun(t *testing.T, got runResult, wantExit, wantDetail int) {
	t.Helper()
	if strings.Contains(got.Stdout, "panic:") {
		t.Fatalf("container panicked:\n%s", got.Stdout)
	}
	if got.ExitCode != wantExit {
		t.Errorf("exit code: got %d want %d\nstdout:\n%s", got.ExitCode, wantExit, got.Stdout)
	}
	if got.Detail != wantDetail {
		t.Errorf("detail: got %d want %d\nstdout:\n%s", got.Detail, wantDetail, got.Stdout)
	}
	if !detailRe.MatchString(got.Stdout) {
		t.Errorf("missing DETAIL line in stdout:\n%s", got.Stdout)
	}
}

func TestGoldenCrossWritesBuy(t *testing.T) {
	requireDocker(t)
	dataDir := newDataDir(t)
	seedProcessSQLite(t, dataDir, []kpiSeed{
		{Symbol: symbol, BarOpen: barOpen() - threeHoursMs, Body: kpiBodyDualSMA(90, 100)},
		{Symbol: symbol, BarOpen: barOpen(), Body: kpiBodyDualSMA(110, 100)},
	})

	res := runContainer(t, dataDir, symbol, candleClose)
	assertRun(t, res, 0, 0)

	actions := readSignals(t, dataDir)
	if len(actions) != 1 {
		t.Fatalf("signal rows: got %d want 1", len(actions))
	}
	a := actions[0]
	if a.Playbook != playbookID || a.Body.Decision != "buy" || a.Body.Rule != "golden_cross" {
		t.Fatalf("got %+v", a)
	}
}

func TestDeathCrossWritesSell(t *testing.T) {
	requireDocker(t)
	dataDir := newDataDir(t)
	seedProcessSQLite(t, dataDir, []kpiSeed{
		{Symbol: symbol, BarOpen: barOpen() - threeHoursMs, Body: kpiBodyDualSMA(110, 100)},
		{Symbol: symbol, BarOpen: barOpen(), Body: kpiBodyDualSMA(90, 100)},
	})
	seedSignal(t, dataDir, symbol, candleClose-threeHoursMs, "buy", "golden_cross")

	res := runContainer(t, dataDir, symbol, candleClose)
	assertRun(t, res, 0, 0)

	actions := readSignals(t, dataDir)
	if len(actions) != 2 {
		t.Fatalf("action rows: got %d want 2", len(actions))
	}
	last := actions[len(actions)-1]
	if last.Body.Decision != "sell" || last.Body.Rule != "death_cross" {
		t.Fatalf("got %+v", last)
	}
}

func TestNoCrossWritesNoTrade(t *testing.T) {
	requireDocker(t)
	dataDir := newDataDir(t)
	seedProcessSQLite(t, dataDir, []kpiSeed{
		{Symbol: symbol, BarOpen: barOpen() - threeHoursMs, Body: kpiBodyDualSMA(110, 100)},
		{Symbol: symbol, BarOpen: barOpen(), Body: kpiBodyDualSMA(112, 100)},
	})
	seedSignal(t, dataDir, symbol, candleClose-threeHoursMs, "buy", "golden_cross")

	res := runContainer(t, dataDir, symbol, candleClose)
	assertRun(t, res, 0, 0)

	actions := readSignals(t, dataDir)
	if len(actions) != 2 {
		t.Fatalf("action rows: got %d want 2", len(actions))
	}
	last := actions[len(actions)-1]
	if last.Body.Decision != "no-trade" || last.Body.Rule != "none" {
		t.Fatalf("got %+v", last)
	}
}

func TestFirstBarNoTrade(t *testing.T) {
	requireDocker(t)
	dataDir := newDataDir(t)
	seedProcessSQLite(t, dataDir, []kpiSeed{
		{Symbol: symbol, BarOpen: barOpen(), Body: kpiBodyDualSMA(110, 100)},
	})

	res := runContainer(t, dataDir, symbol, candleClose)
	assertRun(t, res, 0, 0)

	actions := readSignals(t, dataDir)
	if len(actions) != 1 || actions[0].Body.Decision != "no-trade" {
		t.Fatalf("got %+v", actions)
	}
}

func TestSecondBarStillAboveNoBuy(t *testing.T) {
	requireDocker(t)
	dataDir := newDataDir(t)
	close1 := candleClose
	close2 := candleClose + threeHoursMs
	seedProcessSQLite(t, dataDir, []kpiSeed{
		{Symbol: symbol, BarOpen: close1 - threeHoursMs, Body: kpiBodyDualSMA(110, 100)},
		{Symbol: symbol, BarOpen: close2 - threeHoursMs, Body: kpiBodyDualSMA(112, 100)},
	})

	first := runContainer(t, dataDir, symbol, close1)
	assertRun(t, first, 0, 0)
	actions := readSignals(t, dataDir)
	if len(actions) != 1 || actions[0].Body.Decision != "no-trade" {
		t.Fatalf("first bar: %+v", actions)
	}

	second := runContainer(t, dataDir, symbol, close2)
	assertRun(t, second, 0, 0)
	actions = readSignals(t, dataDir)
	if len(actions) != 2 {
		t.Fatalf("want 2 actions, got %d", len(actions))
	}
	if actions[1].Body.Decision != "no-trade" {
		t.Fatalf("second bar should not buy while still above: %+v", actions[1])
	}
}

func TestDuplicateRunAbortsWithoutSecondWrite(t *testing.T) {
	requireDocker(t)
	dataDir := newDataDir(t)
	seedProcessSQLite(t, dataDir, []kpiSeed{
		{Symbol: symbol, BarOpen: barOpen() - threeHoursMs, Body: kpiBodyDualSMA(90, 100)},
		{Symbol: symbol, BarOpen: barOpen(), Body: kpiBodyDualSMA(110, 100)},
	})

	first := runContainer(t, dataDir, symbol, candleClose)
	assertRun(t, first, 0, 0)

	second := runContainer(t, dataDir, symbol, candleClose)
	assertRun(t, second, 2, 1)

	if len(readSignals(t, dataDir)) != 1 {
		t.Fatal("expected single action row")
	}
}

func TestMissingKPIRowAborts(t *testing.T) {
	requireDocker(t)

	t.Run("empty_table", func(t *testing.T) {
		dataDir := newDataDir(t)
		seedProcessSQLite(t, dataDir, nil)

		res := runContainer(t, dataDir, symbol, candleClose)
		assertRun(t, res, 2, 20)

		if n := len(readSignals(t, dataDir)); n != 0 {
			t.Fatalf("action rows: got %d want 0", n)
		}
	})
}

func TestUnknownPlaybookFails(t *testing.T) {
	requireDocker(t)
	dataDir := newDataDir(t)
	seedProcessSQLite(t, dataDir, []kpiSeed{
		{Symbol: symbol, BarOpen: barOpen(), Body: kpiBodyDualSMA(110, 100)},
	})

	args := []string{
		"run", "--rm",
		"-e", "DATA_DIR=/data",
		"-e", "PLAYBOOK=unknown-strategy",
		"-v", dataDir + ":/data",
		imageTag,
		symbol, fmt.Sprintf("%d", candleClose),
	}
	cmd := exec.Command("docker", args...)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	got := runResult{Stdout: string(out), Detail: parseDetail(string(out))}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			got.ExitCode = exitErr.ExitCode()
		} else {
			t.Fatal(err)
		}
	}
	assertRun(t, got, 1, 40)
}

func TestMissingSMAWritesNoTrade(t *testing.T) {
	requireDocker(t)
	dataDir := newDataDir(t)
	seedProcessSQLite(t, dataDir, []kpiSeed{
		{Symbol: symbol, BarOpen: barOpen(), Body: kpiBodyMovingAverage("sma", 110)},
	})
	res := runContainer(t, dataDir, symbol, candleClose)
	assertRun(t, res, 0, 0)

	actions := readSignals(t, dataDir)
	if len(actions) != 1 || actions[0].Body.Decision != "no-trade" {
		t.Fatalf("got %+v", actions)
	}
}

func kpiBodyMovingAverage(method string, value float64) string {
	body := map[string]any{
		"results": []map[string]any{
			{
				"def": map[string]any{
					"type": "MovingAverage", "method": method, "time_frame_hr": 3,
				},
				"value": value,
				"unit":  "price",
			},
		},
	}
	b, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	return string(b)
}