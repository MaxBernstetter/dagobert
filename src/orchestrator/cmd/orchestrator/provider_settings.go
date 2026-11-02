package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const defaultBinanceBaseURL = "https://data-api.binance.vision"

var secretsFileMu sync.Mutex

// effectiveBaseURL is the base URL a provider reads from: the saved config baseUrl when set, else the
// BINANCE_BASE_URL environment value, else the public default.
func effectiveBaseURL(saved, envValue string) string {
	if v := strings.TrimSpace(saved); v != "" {
		return v
	}
	if v := strings.TrimSpace(envValue); v != "" {
		return v
	}
	return defaultBinanceBaseURL
}

// providerBaseURL resolves the effective base URL of a provider from its stored config. Without a store,
// or when the state cannot be read, it falls back to the environment and the default.
func providerBaseURL(ctx context.Context, id string) string {
	envValue := os.Getenv("BINANCE_BASE_URL")
	if connectorStore == nil {
		return effectiveBaseURL("", envValue)
	}
	rec, ok, err := connectorStore.GetProvider(ctx, id)
	if err != nil || !ok {
		return effectiveBaseURL("", envValue)
	}
	return effectiveBaseURL(rec.Config["baseUrl"], envValue)
}

// jobBaseURL is the Binance base URL handed to job containers: the same precedence as providerBaseURL.
func jobBaseURL() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return providerBaseURL(ctx, binanceProvider.id)
}

// fieldByKey returns the declared field of the provider with that key.
func fieldByKey(key string) (providerField, bool) {
	for _, f := range binanceProvider.fields {
		if f.Key == key {
			return f, true
		}
	}
	return providerField{}, false
}

func isSecretField(f providerField) bool {
	return f.Kind == "secret"
}

// writeProviderSecrets merges the values into DATA_DIR/providers/secrets.json for the provider. The file
// is replaced atomically (temp file, then rename) and keeps mode 0600.
func writeProviderSecrets(providerID string, values map[string]string) error {
	if len(values) == 0 {
		return nil
	}
	secretsFileMu.Lock()
	defer secretsFileMu.Unlock()
	path := filepath.Join(dataDir(), "providers", "secrets.json")
	var doc struct {
		Providers map[string]map[string]string `json:"providers"`
	}
	if raw, err := os.ReadFile(path); err == nil {
		if json.Unmarshal(raw, &doc) != nil {
			return fmt.Errorf("providers/secrets.json is not valid JSON")
		}
	}
	if doc.Providers == nil {
		doc.Providers = map[string]map[string]string{}
	}
	stored := doc.Providers[providerID]
	if stored == nil {
		stored = map[string]string{}
	}
	for k, v := range values {
		stored[k] = strings.TrimSpace(v)
	}
	doc.Providers[providerID] = stored
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".secrets-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(body, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

type providerPatch struct {
	Config  map[string]string `json:"config"`
	Secrets map[string]string `json:"secrets"`
}

// handlePatchProvider saves the non-secret config and the secret values of a provider. Only declared
// fields are accepted. Secrets are never returned. Activation is not changed: a new base URL takes effect
// when the provider is activated again (Refresh assets).
func handlePatchProvider(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("id") != binanceProvider.id {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	store, ok := requireConnectorStore(w)
	if !ok {
		return
	}
	var body providerPatch
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	for key := range body.Config {
		f, declared := fieldByKey(key)
		if !declared || isSecretField(f) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown config key " + key})
			return
		}
	}
	for key := range body.Secrets {
		f, declared := fieldByKey(key)
		if !declared || !isSecretField(f) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown secret key " + key})
			return
		}
	}
	entry, err := loadProviderEntry(r.Context(), store, binanceProvider.id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	merged := map[string]string{}
	for k, v := range entry.Config {
		merged[k] = v
	}
	for k, v := range body.Config {
		merged[k] = strings.TrimSpace(v)
	}
	entry.Config = merged
	if err := writeProviderSecrets(binanceProvider.id, body.Secrets); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	entry.UpdatedAtMs = time.Now().UnixMilli()
	if err := saveProviderEntry(r.Context(), store, binanceProvider.id, entry); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, providerViewFor(entry))
}
