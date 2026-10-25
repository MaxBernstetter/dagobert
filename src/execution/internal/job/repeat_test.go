package job_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/MaxBernstetter/dagobert/src/execution/internal/job"
	"github.com/MaxBernstetter/dagobert/src/execution/internal/store"

	_ "modernc.org/sqlite"
)

func TestRunTwiceAfterFill(t *testing.T) {
	dataDir := t.TempDir()
	seedAction(dataDir)
	srv := newFakeBybit(t)
	defer srv.Close()

	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("PLAYBOOK", "ma-cross.yaml")
	t.Setenv("BYBIT_BASE_URL", srv.URL)
	t.Setenv("BYBIT_API_KEY", "k")
	t.Setenv("BYBIT_API_SECRET", "s")

	exit, detail := job.Run([]string{"BTCUSDT", "1700000000000"})
	if exit != 0 || detail != 0 {
		t.Fatalf("first run: exit=%d detail=%d", exit, detail)
	}
	st, err := store.Open(dataDir)
	if err != nil {
		t.Fatalf("open after first run: %v", err)
	}
	defer st.Close()
	existing, err := st.GetOrderResult("ma-cross.yaml:BTCUSDT:1700000000000")
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if existing.Kind != store.KindFill {
		t.Fatalf("kind: %q", existing.Kind)
	}
	exit, detail = job.Run([]string{"BTCUSDT", "1700000000000"})
	if exit != 2 || detail != 1 {
		t.Fatalf("second run: exit=%d detail=%d", exit, detail)
	}
}

func seedAction(dataDir string) {
	path := filepath.Join(dataDir, "calculation_signal.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	_, _ = db.Exec(`CREATE TABLE action (playbook TEXT, symbol TEXT, candle_close INTEGER, body TEXT, PRIMARY KEY (playbook, symbol, candle_close))`)
	body, _ := json.Marshal(map[string]any{"decision": "buy", "rule": "test", "size": 5.0})
	_, _ = db.Exec(`INSERT INTO action VALUES (?,?,?,?)`, "ma-cross.yaml", "BTCUSDT", 1700000000000, string(body))
}

func newFakeBybit(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v5/order/create":
			_, _ = w.Write([]byte(`{"retCode":0,"result":{"orderId":"oid-1","orderLinkId":"ma-cross.yaml:BTCUSDT:1700000000000"}}`))
		case "/v5/order/history":
			_, _ = w.Write([]byte(`{"retCode":0,"result":{"list":[{"orderId":"oid-1","orderLinkId":"ma-cross.yaml:BTCUSDT:1700000000000","symbol":"BTCUSDC","side":"Buy","orderStatus":"Filled","qty":"5","cumExecQty":"0.001","cumExecValue":"5","avgPrice":"50000","marketUnit":"quoteCoin"}]}}`))
		case "/v5/account/wallet-balance":
			_, _ = w.Write([]byte(`{"retCode":0,"result":{"list":[{"accountType":"UNIFIED","coin":[{"coin":"USDC","walletBalance":"49995"}]}]}}`))
		case "/v5/account/transaction-log":
			_, _ = w.Write([]byte(`{"retCode":0,"result":{"list":[{"currency":"USDC","cashBalance":"49995","transactionTime":2000}]}}`))
		default:
			t.Logf("unexpected path %s", r.URL.Path)
		}
	}))
}
