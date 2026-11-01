package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MaxBernstetter/dagobert/src/experimentapi/pgcatalog"
)

// scratchDB creates a fresh database on POSTGRES_TEST_URL with the current schema, and fails when the
// Postgres is unreachable (the same rule as the experiment system tests).
func scratchDB(t *testing.T) string {
	t.Helper()
	admin := os.Getenv("POSTGRES_TEST_URL")
	if admin == "" {
		admin = "postgres://trading:trading@127.0.0.1:9432/postgres"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ap, err := pgxpool.New(ctx, admin)
	if err == nil {
		err = ap.Ping(ctx)
	}
	if err != nil {
		t.Fatalf("Postgres required for the connector tests (POSTGRES_TEST_URL=%s): %v", admin, err)
	}
	t.Cleanup(ap.Close)
	name := "connector_test_" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	if _, err := ap.Exec(context.Background(), "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create scratch database: %v", err)
	}
	t.Cleanup(func() { _, _ = ap.Exec(context.Background(), "DROP DATABASE "+name+" WITH (FORCE)") })
	cfg := ap.Config().ConnConfig.Copy()
	cfg.Database = name
	url := fmt.Sprintf("postgres://%s:%s@%s:%d/%s", cfg.User, cfg.Password, cfg.Host, cfg.Port, name)
	schema, err := os.ReadFile("../../../../db/sqlc/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	p, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err := p.Exec(context.Background(), string(schema)); err != nil {
		t.Fatal(err)
	}
	return url
}

// fakeBinanceHits counts the exchangeInfo requests the fake served since the test began.
var fakeBinanceHits atomic.Int32

// fakeBinance serves /api/v3/exchangeInfo with three spot markets, or a 500 while fail is set.
func fakeBinance(t *testing.T, fail *atomic.Bool) string {
	t.Helper()
	fakeBinanceHits.Store(0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/exchangeInfo" {
			http.NotFound(w, r)
			return
		}
		fakeBinanceHits.Add(1)
		if fail.Load() {
			http.Error(w, "upstream down", http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, `{"symbols":[
			{"symbol":"BTCUSDT","status":"TRADING","baseAsset":"BTC","quoteAsset":"USDT","permissions":["SPOT"]},
			{"symbol":"ETHUSDT","status":"TRADING","baseAsset":"ETH","quoteAsset":"USDT","permissions":["SPOT"]},
			{"symbol":"ETHBTC","status":"TRADING","baseAsset":"ETH","quoteAsset":"BTC","permissions":["SPOT"]},
			{"symbol":"OLDUSDT","status":"BREAK","baseAsset":"OLD","quoteAsset":"USDT","permissions":["SPOT"]}
		]}`)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

type connectorFixture struct {
	srv   *httptest.Server
	store *pgcatalog.Catalog
}

// newConnectorFixture serves the connector routes over the scratch database. The in-process pairs cache
// is reset, so each test sees its own upstream.
func newConnectorFixture(t *testing.T, upstream string) connectorFixture {
	t.Helper()
	url := scratchDB(t)
	t.Setenv("DATABASE_URL", url)
	store, err := pgcatalog.Open(context.Background())
	if err != nil || store == nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(store.Close)
	t.Setenv("DATA_DIR", t.TempDir())
	t.Setenv("BINANCE_BASE_URL", upstream)
	prevStore := connectorStore
	connectorStore = store
	t.Cleanup(func() { connectorStore = prevStore })
	pairsCacheMu.Lock()
	pairsCache = map[string]pairsCacheEntry{}
	pairsCacheMu.Unlock()

	mux := http.NewServeMux()
	mountConnectors(mux)
	mux.HandleFunc("GET /api/v1/providers/{id}/pairs", handleProviderPairs)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return connectorFixture{srv: srv, store: store}
}

func (f connectorFixture) call(t *testing.T, method, path string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(method, f.srv.URL+path, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("%s %s: body %q: %v", method, path, body, err)
	}
	return resp.StatusCode, out
}

// waitStatus polls the provider until its status is want, or fails after a few seconds.
func (f connectorFixture) waitStatus(t *testing.T, want string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, body := f.call(t, http.MethodGet, "/api/v1/providers/binance")
		if body["status"] == want {
			return body
		}
		if time.Now().After(deadline) {
			t.Fatalf("status %v, want %s", body["status"], want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func pairRows(t *testing.T, body map[string]any) []string {
	t.Helper()
	rows, _ := body["pairs"].([]any)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		m := r.(map[string]any)
		out = append(out, m["base"].(string)+"/"+m["quote"].(string))
	}
	return out
}

func TestConnectorsRequireDatabase(t *testing.T) {
	prev := connectorStore
	connectorStore = nil
	t.Cleanup(func() { connectorStore = prev })
	mux := http.NewServeMux()
	mountConnectors(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/v1/providers")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusServiceUnavailable || !strings.Contains(string(body), "DATABASE_URL required") {
		t.Fatalf("status %d body %s", resp.StatusCode, body)
	}
}

func TestConnectorActivateReadyDeactivate(t *testing.T) {
	var fail atomic.Bool
	f := newConnectorFixture(t, fakeBinance(t, &fail))

	code, list := f.call(t, http.MethodGet, "/api/v1/providers")
	if code != http.StatusOK {
		t.Fatalf("list %d", code)
	}
	provs := list["providers"].([]any)
	first := provs[0].(map[string]any)
	if first["status"] != "inactive" || first["activated"] != false {
		t.Fatalf("never activated: %v", first)
	}
	// The response keys stay as the frontend reads them.
	for _, key := range []string{"id", "label", "description", "fields", "activated", "status", "error", "progress", "config", "secretKeysConfigured", "updatedAtMs"} {
		if _, ok := first[key]; !ok {
			t.Fatalf("provider view lacks %q: %v", key, first)
		}
	}

	code, accepted := f.call(t, http.MethodPost, "/api/v1/providers/binance/activate")
	if code != http.StatusAccepted || accepted["status"] != "setting_up" {
		t.Fatalf("activate %d %v", code, accepted)
	}
	ready := f.waitStatus(t, "ready")
	if ready["activated"] != true || ready["error"] != "" {
		t.Fatalf("ready view: %v", ready)
	}
	progress := ready["progress"].(map[string]any)
	if progress["label"] != "Loaded pairs" || progress["done"] != float64(3) || progress["total"] != float64(3) {
		t.Fatalf("progress: %v", progress)
	}

	_, get := f.call(t, http.MethodGet, "/api/v1/providers/binance")
	if get["status"] != "ready" {
		t.Fatalf("get: %v", get)
	}
	_, pairs := f.call(t, http.MethodGet, "/api/v1/providers/binance/pairs")
	if pairs["provider"] != "binance" {
		t.Fatalf("pairs provider: %v", pairs)
	}
	want := []string{"BTC/USDT", "ETH/BTC", "ETH/USDT"} // the adapter sorts by base, then quote
	if got := pairRows(t, pairs); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("pairs %v want %v", got, want)
	}
	stored, err := f.store.ProviderPairs(context.Background(), "binance")
	if err != nil || len(stored) != 3 {
		t.Fatalf("snapshot rows: %d %v", len(stored), err)
	}

	code, off := f.call(t, http.MethodPost, "/api/v1/providers/binance/deactivate")
	if code != http.StatusOK || off["status"] != "inactive" || off["activated"] != false {
		t.Fatalf("deactivate %d %v", code, off)
	}
	_, after := f.call(t, http.MethodGet, "/api/v1/providers/binance")
	if after["status"] != "inactive" {
		t.Fatalf("after deactivate: %v", after)
	}
}

func TestConnectorFailedSetupKeepsStatusAndError(t *testing.T) {
	var fail atomic.Bool
	fail.Store(true)
	f := newConnectorFixture(t, fakeBinance(t, &fail))

	f.call(t, http.MethodPost, "/api/v1/providers/binance/activate")
	failed := f.waitStatus(t, "failed")
	if failed["activated"] != false {
		t.Fatalf("failed setup must not be activated: %v", failed)
	}
	if msg, _ := failed["error"].(string); !strings.Contains(msg, "status 500") {
		t.Fatalf("error %q", msg)
	}
	// Recovery: the same provider activates once the upstream answers.
	fail.Store(false)
	f.call(t, http.MethodPost, "/api/v1/providers/binance/activate")
	f.waitStatus(t, "ready")
}

func TestConnectorPairsFallBackLiveBeforeSetup(t *testing.T) {
	var fail atomic.Bool
	f := newConnectorFixture(t, fakeBinance(t, &fail))
	code, body := f.call(t, http.MethodGet, "/api/v1/providers/binance/pairs")
	if code != http.StatusOK || len(pairRows(t, body)) != 3 {
		t.Fatalf("live fallback: %d %v", code, body)
	}
	if rows, _ := f.store.ProviderPairs(context.Background(), "binance"); len(rows) != 0 {
		t.Fatalf("the fallback must not store a snapshot: %v", rows)
	}
}
