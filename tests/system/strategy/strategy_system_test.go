package strategy_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

const (
	imageTag     = "trading-copilot-strategy:test"
	candleClose  = int64(1700000000000)
	threeHoursMs = int64(3 * 60 * 60 * 1000)
	symbol       = "BTCUSDT"
)

var (
	dockerOK  bool
	repoRoot  string
	detailRe  = regexp.MustCompile(`(?m)^DETAIL=(\d+)\s*$`)
	playbookPath string
)

func TestMain(m *testing.M) {
	root, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "find repo root: %v\n", err)
		os.Exit(1)
	}
	repoRoot = root
	playbookPath = filepath.Join(repoRoot, "playbooks", "ma-cross.yaml")

	if exec.Command("docker", "version").Run() == nil {
		build := exec.Command(
			"docker", "build",
			"-f", "docker/strategy/Dockerfile",
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
			if _, err := os.Stat(filepath.Join(dir, "docker", "strategy", "Dockerfile")); err == nil {
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

func runContainer(t *testing.T, dataDir, playbookHost, sym string, closeMs int64) runResult {
	t.Helper()
	args := []string{
		"run", "--rm",
		"-e", "DATA_DIR=/data",
		"-e", "PLAYBOOK=" + containerPlaybookPath(playbookHost),
		"-v", dataDir + ":/data",
		"-v", playbookHost + ":" + containerPlaybookPath(playbookHost) + ":ro",
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

func containerPlaybookPath(hostPath string) string {
	return "/playbooks/" + filepath.Base(hostPath)
}

func parseDetail(output string) int {
	m := detailRe.FindStringSubmatch(output)
	if len(m) != 2 {
		return -1
	}
	var d int
	if _, err := fmt.Sscanf(m[1], "%d", &d); err != nil {
		return -1
	}
	return d
}

func newDataDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "strategy-sys-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func seedProcessSQLite(t *testing.T, dataDir string, rows []kpiSeed) {
	t.Helper()
	path := filepath.Join(dataDir, "process.sqlite")
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

func kpiBodyMovingAverage(method string, value float64) string {
	body := map[string]any{
		"results": []map[string]any{
			{
				"def": map[string]any{
					"type":           "MovingAverage",
					"method":         method,
					"time_frame_hr":  3,
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

func readActions(t *testing.T, dataDir string) []actionRow {
	t.Helper()
	path := filepath.Join(dataDir, "strategy.sqlite")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT playbook, symbol, candle_close, body FROM action ORDER BY playbook, symbol, candle_close`)
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
		t.Fatalf("container panicked (expected exit %d detail %d); likely schema compile bug in playbook.LoadAndValidate:\n%s",
			wantExit, wantDetail, got.Stdout)
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

func TestMatchingPlaybookWritesBuy(t *testing.T) {
	requireDocker(t)
	dataDir := newDataDir(t)
	seedProcessSQLite(t, dataDir, []kpiSeed{
		{Symbol: symbol, BarOpen: barOpen(), Body: kpiBodyMovingAverage("sma", 150)},
	})

	res := runContainer(t, dataDir, playbookPath, symbol, candleClose)
	assertRun(t, res, 0, 0)

	actions := readActions(t, dataDir)
	if len(actions) != 1 {
		t.Fatalf("action rows: got %d want 1", len(actions))
	}
	a := actions[0]
	if a.Playbook != "ma-cross.yaml" || a.Symbol != symbol || a.CandleClose != candleClose {
		t.Fatalf("unexpected key: %+v", a)
	}
	if a.Body.Decision != "buy" {
		t.Errorf("decision: got %q want buy", a.Body.Decision)
	}
	if a.Body.Rule != "above" {
		t.Errorf("rule: got %q want above", a.Body.Rule)
	}
	if a.Body.Size == nil || *a.Body.Size != 1 {
		t.Errorf("size: got %v want 1", a.Body.Size)
	}
	if a.Body.Exit == nil || string(a.Body.Exit) == "null" {
		t.Errorf("exit: got %v want time exit", a.Body.Exit)
	}
}

func TestNonMatchingValueWritesNoTrade(t *testing.T) {
	requireDocker(t)
	dataDir := newDataDir(t)
	seedProcessSQLite(t, dataDir, []kpiSeed{
		{Symbol: symbol, BarOpen: barOpen(), Body: kpiBodyMovingAverage("sma", 100)},
	})

	res := runContainer(t, dataDir, playbookPath, symbol, candleClose)
	assertRun(t, res, 0, 0)

	actions := readActions(t, dataDir)
	if len(actions) != 1 {
		t.Fatalf("action rows: got %d want 1", len(actions))
	}
	a := actions[0]
	if a.Body.Decision != "no-trade" {
		t.Errorf("decision: got %q want no-trade", a.Body.Decision)
	}
	if a.Body.Rule != "none" {
		t.Errorf("rule: got %q want none", a.Body.Rule)
	}
	if a.Body.Size != nil {
		t.Errorf("size: got %v want null", a.Body.Size)
	}
	if a.Body.Exit != nil && string(a.Body.Exit) != "null" {
		t.Errorf("exit: got %v want null", a.Body.Exit)
	}
}

func TestDuplicateRunAbortsWithoutSecondWrite(t *testing.T) {
	requireDocker(t)
	dataDir := newDataDir(t)
	seedProcessSQLite(t, dataDir, []kpiSeed{
		{Symbol: symbol, BarOpen: barOpen(), Body: kpiBodyMovingAverage("sma", 150)},
	})

	first := runContainer(t, dataDir, playbookPath, symbol, candleClose)
	assertRun(t, first, 0, 0)

	second := runContainer(t, dataDir, playbookPath, symbol, candleClose)
	assertRun(t, second, 2, 1)

	actions := readActions(t, dataDir)
	if len(actions) != 1 {
		t.Fatalf("action rows: got %d want 1", len(actions))
	}
	if actions[0].Body.Decision != "buy" || actions[0].Body.Rule != "above" {
		t.Fatalf("expected original buy row, got %+v", actions[0].Body)
	}
}

func TestMissingKPIRowAborts(t *testing.T) {
	requireDocker(t)

	t.Run("empty_table", func(t *testing.T) {
		dataDir := newDataDir(t)
		seedProcessSQLite(t, dataDir, nil)

		res := runContainer(t, dataDir, playbookPath, symbol, candleClose)
		assertRun(t, res, 2, 20)

		if n := len(readActions(t, dataDir)); n != 0 {
			t.Fatalf("action rows: got %d want 0", n)
		}
	})

	t.Run("other_symbol", func(t *testing.T) {
		dataDir := newDataDir(t)
		seedProcessSQLite(t, dataDir, []kpiSeed{
			{Symbol: "ETHUSDT", BarOpen: barOpen(), Body: kpiBodyMovingAverage("sma", 150)},
		})

		res := runContainer(t, dataDir, playbookPath, symbol, candleClose)
		assertRun(t, res, 2, 20)

		if n := len(readActions(t, dataDir)); n != 0 {
			t.Fatalf("action rows: got %d want 0", n)
		}
	})
}

func TestInvalidPlaybookFailsSchema(t *testing.T) {
	requireDocker(t)
	dataDir := newDataDir(t)
	seedProcessSQLite(t, dataDir, []kpiSeed{
		{Symbol: symbol, BarOpen: barOpen(), Body: kpiBodyMovingAverage("sma", 150)},
	})

	invalidPath := filepath.Join(newDataDir(t), "invalid-playbook.yaml")
	content := `# yaml-language-server: $schema=../strategy/schema/playbook.schema.json
name: invalid
rules:
  - id: bad
    when:
      def:
        type: MovingAverage
        method: sma
        time_frame_hr: 3
      op: gt
      value: 100
    decision: hold
    size: 1
    exit:
      kind: time
      bars: 1
`
	if err := os.WriteFile(invalidPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	res := runContainer(t, dataDir, invalidPath, symbol, candleClose)
	assertRun(t, res, 1, 40)

	if n := len(readActions(t, dataDir)); n != 0 {
		t.Fatalf("action rows: got %d want 0", n)
	}
}

func TestIndicatorDefObjectNotNickname(t *testing.T) {
	requireDocker(t)
	dataDir := newDataDir(t)
	// Value would match sma gt 100, but KPI def is ema — must not match trade rules.
	seedProcessSQLite(t, dataDir, []kpiSeed{
		{Symbol: symbol, BarOpen: barOpen(), Body: kpiBodyMovingAverage("ema", 150)},
	})

	res := runContainer(t, dataDir, playbookPath, symbol, candleClose)
	assertRun(t, res, 0, 0)

	actions := readActions(t, dataDir)
	if len(actions) != 1 {
		t.Fatalf("action rows: got %d want 1", len(actions))
	}
	if actions[0].Body.Decision != "no-trade" || actions[0].Body.Rule != "none" {
		t.Fatalf("expected no-trade/none, got decision=%q rule=%q", actions[0].Body.Decision, actions[0].Body.Rule)
	}
}

func TestMatchingPlaybookWritesSell(t *testing.T) {
	requireDocker(t)
	dataDir := newDataDir(t)
	seedProcessSQLite(t, dataDir, []kpiSeed{
		{Symbol: symbol, BarOpen: barOpen(), Body: kpiBodyMovingAverage("sma", 50)},
	})

	res := runContainer(t, dataDir, playbookPath, symbol, candleClose)
	assertRun(t, res, 0, 0)

	actions := readActions(t, dataDir)
	if len(actions) != 1 {
		t.Fatalf("action rows: got %d want 1", len(actions))
	}
	a := actions[0]
	if a.Body.Decision != "sell" || a.Body.Rule != "below" {
		t.Fatalf("got decision=%q rule=%q want sell/below", a.Body.Decision, a.Body.Rule)
	}
	if a.Body.Size == nil || *a.Body.Size != 1 {
		t.Errorf("size: got %v want 1", a.Body.Size)
	}
}

func TestNicknameDefDoesNotMatch(t *testing.T) {
	requireDocker(t)
	dataDir := newDataDir(t)
	body := `{"results":[{"def":"sma_3h","value":150,"unit":"price"}]}`
	seedProcessSQLite(t, dataDir, []kpiSeed{
		{Symbol: symbol, BarOpen: barOpen(), Body: body},
	})

	res := runContainer(t, dataDir, playbookPath, symbol, candleClose)
	assertRun(t, res, 0, 0)

	actions := readActions(t, dataDir)
	if len(actions) != 1 {
		t.Fatalf("action rows: got %d want 1", len(actions))
	}
	if actions[0].Body.Decision != "no-trade" || actions[0].Body.Rule != "none" {
		t.Fatalf("nickname def must not match sma rule: decision=%q rule=%q", actions[0].Body.Decision, actions[0].Body.Rule)
	}
}
