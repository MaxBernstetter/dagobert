package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type providerField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Kind        string `json:"kind"`
	Description string `json:"description,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Default     string `json:"default,omitempty"`
}

type providerProgress struct {
	Label string `json:"label"`
	Done  *int   `json:"done"`
	Total *int   `json:"total"`
}

type providerEntry struct {
	Activated   bool              `json:"activated"`
	Status      string            `json:"status"`
	Error       string            `json:"error"`
	Progress    *providerProgress `json:"progress"`
	Config      map[string]string `json:"config"`
	UpdatedAtMs int64             `json:"updatedAtMs"`
}

type providerView struct {
	ID                   string            `json:"id"`
	Label                string            `json:"label"`
	Description          string            `json:"description,omitempty"`
	Fields               []providerField   `json:"fields"`
	Activated            bool              `json:"activated"`
	Status               string            `json:"status"`
	Error                string            `json:"error"`
	Progress             *providerProgress `json:"progress"`
	Config               map[string]string `json:"config"`
	SecretKeysConfigured []string          `json:"secretKeysConfigured"`
	UpdatedAtMs          int64             `json:"updatedAtMs"`
}

var binanceProvider = struct {
	id, label, description string
	fields                 []providerField
}{
	id:          "binance",
	label:       "Binance",
	description: "Public spot klines (no API key).",
	fields: []providerField{{
		Key:         "baseUrl",
		Label:       "Base URL",
		Kind:        "string",
		Description: "Public data API host for klines and exchangeInfo.",
		Placeholder: "https://data-api.binance.vision",
		Default:     "https://data-api.binance.vision",
	}},
}

var providerSetupMu sync.Mutex

func mountConnectors(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/providers", handleListProviders)
	mux.HandleFunc("GET /api/v1/providers/{id}", handleGetProvider)
	mux.HandleFunc("POST /api/v1/providers/{id}/activate", handleActivateProvider)
	mux.HandleFunc("POST /api/v1/providers/{id}/deactivate", handleDeactivateProvider)
}

func dataDir() string {
	if d := strings.TrimSpace(os.Getenv("LIVE_DIR")); d != "" {
		return d
	}
	if d := strings.TrimSpace(os.Getenv("DATA_DIR")); d != "" {
		return d
	}
	return "data"
}

func catalogFile() string {
	return filepath.Join(dataDir(), "providers", "catalog.json")
}

func readCatalog() map[string]providerEntry {
	raw, err := os.ReadFile(catalogFile())
	if err != nil {
		return map[string]providerEntry{}
	}
	var doc struct {
		Providers map[string]providerEntry `json:"providers"`
	}
	if json.Unmarshal(raw, &doc) != nil || doc.Providers == nil {
		return map[string]providerEntry{}
	}
	return doc.Providers
}

func writeCatalog(providers map[string]providerEntry) error {
	dir := filepath.Dir(catalogFile())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(struct {
		Providers map[string]providerEntry `json:"providers"`
	}{Providers: providers}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(catalogFile(), append(body, '\n'), 0o644)
}

func secretKeys(providerID string) []string {
	raw, err := os.ReadFile(filepath.Join(dataDir(), "providers", "secrets.json"))
	if err != nil {
		return []string{}
	}
	var doc struct {
		Providers map[string]map[string]string `json:"providers"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return []string{}
	}
	keys := doc.Providers[providerID]
	out := make([]string, 0, len(keys))
	for k, v := range keys {
		if strings.TrimSpace(v) != "" {
			out = append(out, k)
		}
	}
	return out
}

func providerViewFor(entry providerEntry) providerView {
	if entry.Config == nil {
		entry.Config = map[string]string{"baseUrl": binanceProvider.fields[0].Default}
	}
	if entry.Status == "" {
		entry.Status = "inactive"
	}
	secrets := secretKeys(binanceProvider.id)
	if secrets == nil {
		secrets = []string{}
	}
	return providerView{
		ID:                   binanceProvider.id,
		Label:                binanceProvider.label,
		Description:          binanceProvider.description,
		Fields:               binanceProvider.fields,
		Activated:            entry.Activated,
		Status:               entry.Status,
		Error:                entry.Error,
		Progress:             entry.Progress,
		Config:               entry.Config,
		SecretKeysConfigured: secrets,
		UpdatedAtMs:          entry.UpdatedAtMs,
	}
}

func handleListProviders(w http.ResponseWriter, r *http.Request) {
	entry := readCatalog()[binanceProvider.id]
	writeJSON(w, http.StatusOK, map[string]any{"providers": []providerView{providerViewFor(entry)}})
}

func handleGetProvider(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("id") != binanceProvider.id {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, providerViewFor(readCatalog()[binanceProvider.id]))
}

func handleDeactivateProvider(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("id") != binanceProvider.id {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	catalog := readCatalog()
	entry := catalog[binanceProvider.id]
	entry.Activated = false
	entry.Status = "inactive"
	entry.Error = ""
	entry.Progress = nil
	entry.UpdatedAtMs = time.Now().UnixMilli()
	if entry.Config == nil {
		entry.Config = map[string]string{}
	}
	catalog[binanceProvider.id] = entry
	if err := writeCatalog(catalog); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, providerViewFor(entry))
}

func handleActivateProvider(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("id") != binanceProvider.id {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	catalog := readCatalog()
	entry := catalog[binanceProvider.id]
	entry.Status = "setting_up"
	entry.Activated = false
	entry.Error = ""
	zero := 0
	entry.Progress = &providerProgress{Label: "Fetching markets", Done: &zero, Total: nil}
	entry.UpdatedAtMs = time.Now().UnixMilli()
	if entry.Config == nil {
		entry.Config = map[string]string{"baseUrl": binanceProvider.fields[0].Default}
	}
	catalog[binanceProvider.id] = entry
	if err := writeCatalog(catalog); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	go finishProviderSetup()
	writeJSON(w, http.StatusAccepted, providerViewFor(entry))
}

func finishProviderSetup() {
	providerSetupMu.Lock()
	defer providerSetupMu.Unlock()
	catalog := readCatalog()
	entry := catalog[binanceProvider.id]
	pairs, err := listProviderPairs(context.Background(), binanceProvider.id)
	if err != nil {
		entry.Status = "failed"
		entry.Activated = false
		entry.Error = err.Error()
		entry.UpdatedAtMs = time.Now().UnixMilli()
		catalog[binanceProvider.id] = entry
		_ = writeCatalog(catalog)
		return
	}
	n := len(pairs)
	entry.Activated = true
	entry.Status = "ready"
	entry.Error = ""
	entry.Progress = &providerProgress{Label: "Loaded pairs", Done: &n, Total: &n}
	entry.UpdatedAtMs = time.Now().UnixMilli()
	catalog[binanceProvider.id] = entry
	_ = writeCatalog(catalog)
	dir := filepath.Join(dataDir(), "providers", binanceProvider.id)
	_ = os.MkdirAll(dir, 0o755)
	rows := make([]map[string]string, 0, len(pairs))
	for _, p := range pairs {
		rows = append(rows, map[string]string{"base": p.Base, "quote": p.Quote})
	}
	body, _ := json.MarshalIndent(map[string]any{"pairs": rows}, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, "pairs.json"), append(body, '\n'), 0o644)
}
