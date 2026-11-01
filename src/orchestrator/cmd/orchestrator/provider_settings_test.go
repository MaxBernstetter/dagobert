package main

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestEffectiveBaseURLPrecedence(t *testing.T) {
	cases := []struct {
		name, saved, env, want string
	}{
		{"saved wins over env", "https://saved.example", "https://env.example", "https://saved.example"},
		{"env when nothing saved", "", "https://env.example", "https://env.example"},
		{"default when neither", "", "", defaultBinanceBaseURL},
		{"blank saved falls through to env", "   ", "https://env.example", "https://env.example"},
		{"blank everywhere is the default", " ", " ", defaultBinanceBaseURL},
		{"saved value is trimmed", "  https://saved.example/ ", "", "https://saved.example/"},
	}
	for _, c := range cases {
		if got := effectiveBaseURL(c.saved, c.env); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

// withSecretField declares one secret field for the test and restores the provider afterwards.
func withSecretField(t *testing.T) {
	t.Helper()
	prev := binanceProvider.fields
	binanceProvider.fields = append(append([]providerField{}, prev...), providerField{
		Key: "apiKey", Label: "API key", Kind: "secret",
	})
	t.Cleanup(func() { binanceProvider.fields = prev })
}

func (f connectorFixture) patch(t *testing.T, path, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPatch, f.srv.URL+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

func TestPatchProviderConfigAndSecrets(t *testing.T) {
	withSecretField(t)
	var fail atomic.Bool
	upstream := fakeBinance(t, &fail)
	f := newConnectorFixture(t, upstream)
	const secret = "s3cr3t-value-123"

	if code, _ := f.patch(t, "/api/v1/providers/nope", `{"config":{}}`); code != http.StatusNotFound {
		t.Fatalf("unknown provider: %d", code)
	}
	if code, body := f.patch(t, "/api/v1/providers/binance", `{"config":{"foo":"x"}}`); code != http.StatusBadRequest || !strings.Contains(body, "unknown config key foo") {
		t.Fatalf("unknown config key: %d %s", code, body)
	}
	if code, _ := f.patch(t, "/api/v1/providers/binance", `{"config":{"apiKey":"x"}}`); code != http.StatusBadRequest {
		t.Fatalf("a secret field sent as config must be rejected: %d", code)
	}
	if code, _ := f.patch(t, "/api/v1/providers/binance", `{"secrets":{"baseUrl":"x"}}`); code != http.StatusBadRequest {
		t.Fatalf("a non-secret field sent as secret must be rejected: %d", code)
	}

	// Activate first: saving config must not flip the provider to inactive.
	f.call(t, http.MethodPost, "/api/v1/providers/binance/activate")
	f.waitStatus(t, "ready")

	// The saved base URL is what activation reads, so the bogus env value below must not matter.
	t.Setenv("BINANCE_BASE_URL", "http://127.0.0.1:1")
	code, saved := f.patch(t, "/api/v1/providers/binance", `{"config":{"baseUrl":"`+upstream+`"}}`)
	if code != http.StatusOK {
		t.Fatalf("save config: %d %s", code, saved)
	}
	if !strings.Contains(saved, `"baseUrl":"`+upstream+`"`) || !strings.Contains(saved, `"status":"ready"`) {
		t.Fatalf("view after save: %s", saved)
	}
	// Re-activation reads the markets from the saved base URL: the exchange fake must see the request.
	before := fakeBinanceHits.Load()
	f.call(t, http.MethodPost, "/api/v1/providers/binance/activate")
	f.waitStatus(t, "ready")
	if fakeBinanceHits.Load() != before+1 {
		t.Fatalf("activation did not read the saved base URL (hits %d -> %d)", before, fakeBinanceHits.Load())
	}

	code, body := f.patch(t, "/api/v1/providers/binance", `{"config":{"baseUrl":"`+upstream+`"},"secrets":{"apiKey":"`+secret+`"}}`)
	if code != http.StatusOK || strings.Contains(body, secret) {
		t.Fatalf("save secret: %d, secret in response=%v", code, strings.Contains(body, secret))
	}

	_, get := f.call(t, http.MethodGet, "/api/v1/providers/binance")
	if get["status"] != "ready" {
		t.Fatalf("saving config changed the status: %v", get)
	}
	if cfg := get["config"].(map[string]any); cfg["baseUrl"] != upstream {
		t.Fatalf("persisted config: %v", cfg)
	}
	keys := get["secretKeysConfigured"].([]any)
	if len(keys) != 1 || keys[0] != "apiKey" {
		t.Fatalf("secretKeysConfigured %v", keys)
	}

	// The secret is only in the file, with mode 0600, and in no response.
	path := filepath.Join(os.Getenv("DATA_DIR"), "providers", "secrets.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("secrets.json mode %v want 0600", info.Mode().Perm())
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), secret) {
		t.Fatal("the secret was not written to secrets.json")
	}
	for _, path := range []string{"/api/v1/providers", "/api/v1/providers/binance", "/api/v1/providers/binance/pairs"} {
		resp, err := http.Get(f.srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if strings.Contains(string(raw), secret) {
			t.Fatalf("secret leaked in %s", path)
		}
	}
}

// TestActivationBypassesPairsCache: activation asks the exchange even when a listing is cached. A plain
// listing of the same provider is served from the cache.
func TestActivationBypassesPairsCache(t *testing.T) {
	var fail atomic.Bool
	f := newConnectorFixture(t, fakeBinance(t, &fail))

	f.call(t, http.MethodPost, "/api/v1/providers/binance/activate")
	f.waitStatus(t, "ready")
	if hits := fakeBinanceHits.Load(); hits != 1 {
		t.Fatalf("activation requests %d want 1", hits)
	}
	f.call(t, http.MethodGet, "/api/v1/providers/binance/pairs")
	if hits := fakeBinanceHits.Load(); hits != 1 {
		t.Fatalf("pairs listing must come from the cache (hits %d)", hits)
	}
	fail.Store(true)
	f.call(t, http.MethodPost, "/api/v1/providers/binance/activate")
	f.waitStatus(t, "failed")
	if hits := fakeBinanceHits.Load(); hits != 2 {
		t.Fatalf("second activation must query the exchange (hits %d)", hits)
	}
}
