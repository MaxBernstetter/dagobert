package execution_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	_ "modernc.org/sqlite"
)

const (
	imageTag    = "trading-copilot-execution:test"
	candleClose = int64(1700000000000)
	symbol      = "BTCUSDT"
	playbook    = "ma-cross.yaml"
)

var (
	dockerOK bool
	repoRoot string
	detailRe = regexp.MustCompile(`(?m)^DETAIL=(\d+)\s*$`)
)

type fakeBybit struct {
	server           *httptest.Server
	createCount      atomic.Int32
	historyStatus    atomic.Value // string
	createRetCode    atomic.Int32
	createHTTPStatus atomic.Int32
	historyHTTP500   atomic.Bool
}

func (f *fakeBybit) URL() string {
	return f.server.URL
}

func newFakeBybit() *fakeBybit {
	f := &fakeBybit{}
	f.historyStatus.Store("Filled")
	f.createRetCode.Store(int32(0))
	f.createHTTPStatus.Store(int32(200))
	mux := http.NewServeMux()
	mux.HandleFunc("/v5/order/create", f.handleCreate)
	mux.HandleFunc("/v5/order/history", f.handleHistory)
	mux.HandleFunc("/v5/account/wallet-balance", f.handleWalletBalance)
	mux.HandleFunc("/v5/account/transaction-log", f.handleTransactionLog)
	f.server = httptest.NewServer(mux)
	return f
}

func (f *fakeBybit) Close() { f.server.Close() }

func (f *fakeBybit) handleCreate(w http.ResponseWriter, r *http.Request) {
	f.createCount.Add(1)
	if int(f.createHTTPStatus.Load()) >= 500 {
		http.Error(w, "error", int(f.createHTTPStatus.Load()))
		return
	}
	body, _ := io.ReadAll(r.Body)
	var req map[string]string
	_ = json.Unmarshal(body, &req)
	if req["marketUnit"] == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	code := f.createRetCode.Load()
	if code != 0 {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fmt.Sprintf(`{"retCode":%d,"retMsg":"rejected","result":{}}`, code)))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	link := req["orderLinkId"]
	_, _ = w.Write([]byte(fmt.Sprintf(`{"retCode":0,"retMsg":"OK","result":{"orderId":"oid-1","orderLinkId":%q}}`, link)))
}

func (f *fakeBybit) handleHistory(w http.ResponseWriter, r *http.Request) {
	if f.historyHTTP500.Load() {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	status, _ := f.historyStatus.Load().(string)
	if status == "" {
		status = "Filled"
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(fmt.Sprintf(`{"retCode":0,"retMsg":"OK","result":{"list":[{
		"orderId":"oid-1",
		"orderLinkId":"%s",
		"symbol":"BTCUSDC",
		"side":"Buy",
		"orderType":"Market",
		"orderStatus":%q,
		"qty":"5",
		"cumExecQty":"0.001",
		"cumExecValue":"5",
		"avgPrice":"50000",
		"marketUnit":"quoteCoin"
	}]}}`, r.URL.Query().Get("orderId"), status)))
}

func (f *fakeBybit) handleWalletBalance(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"retCode":0,"retMsg":"OK","result":{"list":[{"accountType":"UNIFIED","coin":[
		{"coin":"USDC","walletBalance":"49995","equity":"49995"},
		{"coin":"BTC","walletBalance":"1.001","equity":"1.001"}
	]}]}}`))
}

func (f *fakeBybit) handleTransactionLog(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"retCode":0,"retMsg":"OK","result":{"list":[
		{"currency":"USDC","cashBalance":"49995","transactionTime":2000},
		{"currency":"USDC","cashBalance":"50000","transactionTime":1000},
		{"currency":"BTC","cashBalance":"1.001","transactionTime":2000}
	]}}`))
}

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
			"-f", "docker/execution/Dockerfile",
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
			if _, err := os.Stat(filepath.Join(dir, "docker", "execution", "Dockerfile")); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("repo root not found")
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

type runResult struct {
	ExitCode int
	Detail   int
	Stdout   string
}

func runContainer(t *testing.T, dataDir string, fake *fakeBybit, sym string, closeMs int64, extraEnv ...string) runResult {
	t.Helper()
	env := []string{
		"DATA_DIR=/data",
		"PLAYBOOK=" + playbook,
		"BYBIT_BASE_URL=" + fake.URL(),
		"BYBIT_API_KEY=test-key",
		"BYBIT_API_SECRET=test-secret",
	}
	env = append(env, extraEnv...)
	args := []string{"run", "--rm", "--network", "host"}
	for _, e := range env {
		args = append(args, "-e", e)
	}
	args = append(args, "-v", dataDir+":/data", imageTag)
	if sym != "" {
		args = append(args, sym, fmt.Sprintf("%d", closeMs))
	} else {
		args = append(args, fmt.Sprintf("%d", closeMs))
	}
	cmd := exec.Command("docker", args...)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	stdout := string(out)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return runResult{ExitCode: exitErr.ExitCode(), Detail: parseDetail(stdout), Stdout: stdout}
		}
		t.Fatalf("docker run: %v\n%s", err, stdout)
	}
	return runResult{ExitCode: 0, Detail: parseDetail(stdout), Stdout: stdout}
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
	dir, err := os.MkdirTemp("", "execution-sys-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func seedAction(t *testing.T, dataDir, decision string, size *float64) {
	t.Helper()
	path := filepath.Join(dataDir, "calculation_signal.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS action (
  playbook TEXT NOT NULL,
  symbol TEXT NOT NULL,
  candle_close INTEGER NOT NULL,
  body TEXT NOT NULL,
  PRIMARY KEY (playbook, symbol, candle_close)
);`); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"decision": decision, "rule": "test"}
	if size != nil {
		body["size"] = *size
	}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		`INSERT INTO action (playbook, symbol, candle_close, body) VALUES (?, ?, ?, ?)`,
		playbook, symbol, candleClose, string(b),
	)
	if err != nil {
		t.Fatal(err)
	}
}

func assertRun(t *testing.T, got runResult, wantExit, wantDetail int) {
	t.Helper()
	if got.ExitCode != wantExit {
		t.Errorf("exit code: got %d want %d\nstdout:\n%s", got.ExitCode, wantExit, got.Stdout)
	}
	if got.Detail != wantDetail {
		t.Errorf("detail: got %d want %d\nstdout:\n%s", got.Detail, wantDetail, got.Stdout)
	}
	if !detailRe.MatchString(got.Stdout) {
		t.Errorf("missing DETAIL line:\n%s", got.Stdout)
	}
}

func orderKind(t *testing.T, dataDir string) (string, json.RawMessage) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dataDir, "execution.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var kind, body string
	link := playbook + ":" + symbol + ":" + fmt.Sprintf("%d", candleClose)
	err = db.QueryRow(`SELECT kind, body FROM order_result WHERE order_link_id = ?`, link).Scan(&kind, &body)
	if err != nil {
		t.Fatal(err)
	}
	return kind, json.RawMessage(body)
}

func TestBuyFill(t *testing.T) {
	requireDocker(t)
	fake := newFakeBybit()
	t.Cleanup(fake.Close)
	dataDir := newDataDir(t)
	size := 5.0
	seedAction(t, dataDir, "buy", &size)

	res := runContainer(t, dataDir, fake, symbol, candleClose)
	assertRun(t, res, 0, 0)
	if fake.createCount.Load() != 1 {
		t.Fatalf("create calls: got %d want 1", fake.createCount.Load())
	}
	kind, _ := orderKind(t, dataDir)
	if kind != "fill" {
		t.Fatalf("kind: got %q want fill", kind)
	}
}

func TestBuyFillRepeatAborts(t *testing.T) {
	requireDocker(t)
	fake := newFakeBybit()
	t.Cleanup(fake.Close)
	dataDir := newDataDir(t)
	size := 5.0
	seedAction(t, dataDir, "buy", &size)

	assertRun(t, runContainer(t, dataDir, fake, symbol, candleClose), 0, 0)
	firstCreates := fake.createCount.Load()
	res := runContainer(t, dataDir, fake, symbol, candleClose)
	assertRun(t, res, 2, 1)
	if fake.createCount.Load() != firstCreates {
		t.Fatalf("expected no second create, count=%d", fake.createCount.Load())
	}
}

func TestNoTrade(t *testing.T) {
	requireDocker(t)
	fake := newFakeBybit()
	t.Cleanup(fake.Close)
	dataDir := newDataDir(t)
	seedAction(t, dataDir, "no-trade", nil)

	res := runContainer(t, dataDir, fake, symbol, candleClose)
	assertRun(t, res, 0, 2)
	if fake.createCount.Load() != 0 {
		t.Fatalf("create calls: got %d want 0", fake.createCount.Load())
	}
	kind, _ := orderKind(t, dataDir)
	if kind != "no-trade" {
		t.Fatalf("kind: got %q want no-trade", kind)
	}
	db, err := sql.Open("sqlite", filepath.Join(dataDir, "execution.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM balance`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("expected balance rows")
	}
}

func TestSellBaseCoin(t *testing.T) {
	requireDocker(t)
	fake := newFakeBybit()
	t.Cleanup(fake.Close)
	var lastCreateBody string
	fake.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v5/order/create":
			fake.createCount.Add(1)
			b, _ := io.ReadAll(r.Body)
			lastCreateBody = string(b)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"retCode":0,"retMsg":"OK","result":{"orderId":"oid-sell","orderLinkId":"x"}}`))
		case "/v5/order/history":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"retCode":0,"retMsg":"OK","result":{"list":[{
				"orderId":"oid-sell","orderLinkId":"x","symbol":"BTCUSDC","side":"Sell","orderType":"Market",
				"orderStatus":"Filled","qty":"0.001","cumExecQty":"0.001","cumExecValue":"50","avgPrice":"50000","marketUnit":"baseCoin"
			}]}}`))
		case "/v5/account/wallet-balance":
			fake.handleWalletBalance(w, r)
		case "/v5/account/transaction-log":
			fake.handleTransactionLog(w, r)
		default:
			http.NotFound(w, r)
		}
	})

	dataDir := newDataDir(t)
	size := 0.001
	seedAction(t, dataDir, "sell", &size)
	res := runContainer(t, dataDir, fake, symbol, candleClose)
	assertRun(t, res, 0, 0)
	if !strings.Contains(lastCreateBody, `"marketUnit":"baseCoin"`) {
		t.Fatalf("expected baseCoin create body, got %s", lastCreateBody)
	}
}

func TestMissingActionNoHTTP(t *testing.T) {
	requireDocker(t)
	fake := newFakeBybit()
	t.Cleanup(fake.Close)
	dataDir := newDataDir(t)

	res := runContainer(t, dataDir, fake, symbol, candleClose)
	assertRun(t, res, 2, 20)
	if fake.createCount.Load() != 0 {
		t.Fatalf("expected no Bybit calls")
	}
}

func TestMissingSymbolArg(t *testing.T) {
	requireDocker(t)
	fake := newFakeBybit()
	t.Cleanup(fake.Close)
	dataDir := newDataDir(t)
	size := 5.0
	seedAction(t, dataDir, "buy", &size)

	args := []string{
		"run", "--rm", "--network", "host",
		"-e", "DATA_DIR=/data",
		"-e", "PLAYBOOK=" + playbook,
		"-e", "BYBIT_BASE_URL=" + fake.URL(),
		"-v", dataDir + ":/data",
		imageTag,
		fmt.Sprintf("%d", candleClose),
	}
	cmd := exec.Command("docker", args...)
	cmd.Dir = repoRoot
	out, _ := cmd.CombinedOutput()
	stdout := string(out)
	if !strings.Contains(stdout, "DETAIL=10") {
		t.Fatalf("expected DETAIL=10, got:\n%s", stdout)
	}
}

func TestPermanentRejection(t *testing.T) {
	requireDocker(t)
	fake := newFakeBybit()
	t.Cleanup(fake.Close)
	fake.createRetCode.Store(10001)
	dataDir := newDataDir(t)
	size := 5.0
	seedAction(t, dataDir, "buy", &size)

	res := runContainer(t, dataDir, fake, symbol, candleClose)
	assertRun(t, res, 1, 40)
	kind, _ := orderKind(t, dataDir)
	if kind != "rejection" {
		t.Fatalf("kind: got %q want rejection", kind)
	}
	assertRun(t, runContainer(t, dataDir, fake, symbol, candleClose), 2, 1)
}

func TestPendingThenFilled(t *testing.T) {
	requireDocker(t)
	fake := newFakeBybit()
	t.Cleanup(fake.Close)
	fake.historyStatus.Store("New")
	dataDir := newDataDir(t)
	size := 5.0
	seedAction(t, dataDir, "buy", &size)

	res := runContainer(t, dataDir, fake, symbol, candleClose)
	assertRun(t, res, 2, 31)
	if fake.createCount.Load() != 1 {
		t.Fatalf("create calls: got %d want 1", fake.createCount.Load())
	}
	kind, _ := orderKind(t, dataDir)
	if kind != "pending" {
		t.Fatalf("kind: got %q want pending", kind)
	}

	fake.historyStatus.Store("Filled")
	res2 := runContainer(t, dataDir, fake, symbol, candleClose)
	assertRun(t, res2, 0, 0)
	if fake.createCount.Load() != 1 {
		t.Fatalf("second run should not create, count=%d", fake.createCount.Load())
	}
	kind, _ = orderKind(t, dataDir)
	if kind != "fill" {
		t.Fatalf("kind after poll: got %q want fill", kind)
	}
}

func TestCreateHTTP500RetryExhausted(t *testing.T) {
	requireDocker(t)
	fake := newFakeBybit()
	t.Cleanup(fake.Close)
	fake.createHTTPStatus.Store(500)
	dataDir := newDataDir(t)
	size := 5.0
	seedAction(t, dataDir, "buy", &size)

	res := runContainer(t, dataDir, fake, symbol, candleClose, "MAX_RETRIES=1")
	assertRun(t, res, 1, 30)
	if _, err := os.Stat(filepath.Join(dataDir, "execution.sqlite")); err == nil {
		db, err := sql.Open("sqlite", filepath.Join(dataDir, "execution.sqlite"))
		if err == nil {
			defer db.Close()
			var n int
			_ = db.QueryRow(`SELECT COUNT(*) FROM order_result`).Scan(&n)
			if n != 0 {
				t.Fatalf("expected no order_result rows, got %d", n)
			}
		}
	}
}
