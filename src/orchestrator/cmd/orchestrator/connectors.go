package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/MaxBernstetter/dagobert/src/experimentapi/pgcatalog"
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
	mux.HandleFunc("PATCH /api/v1/providers/{id}", handlePatchProvider)
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

// connectorStore is the connector state in Postgres. serveBackfill sets it when DATABASE_URL is set;
// the connector routes answer 503 without it.
var connectorStore *pgcatalog.Catalog

const connectorDBRequired = "DATABASE_URL required: connector state is stored in Postgres"

// requireConnectorStore returns the store, or writes the 503 for a missing DATABASE_URL.
func requireConnectorStore(w http.ResponseWriter) (*pgcatalog.Catalog, bool) {
	if connectorStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": connectorDBRequired})
		return nil, false
	}
	return connectorStore, true
}

// loadProviderEntry reads the stored state of the provider; a provider never saved is the zero entry.
func loadProviderEntry(ctx context.Context, store *pgcatalog.Catalog, id string) (providerEntry, error) {
	rec, ok, err := store.GetProvider(ctx, id)
	if err != nil || !ok {
		return providerEntry{}, err
	}
	entry := providerEntry{
		Activated:   rec.Activated,
		Status:      rec.Status,
		Error:       rec.Error,
		Config:      rec.Config,
		UpdatedAtMs: rec.UpdatedAtMs,
	}
	if rec.ProgressLabel != nil {
		entry.Progress = &providerProgress{Label: *rec.ProgressLabel, Done: rec.ProgressDone, Total: rec.ProgressTotal}
	}
	return entry, nil
}

func saveProviderEntry(ctx context.Context, store *pgcatalog.Catalog, id string, entry providerEntry) error {
	rec := pgcatalog.ProviderRecord{
		ID:          id,
		Activated:   entry.Activated,
		Status:      entry.Status,
		Error:       entry.Error,
		Config:      entry.Config,
		UpdatedAtMs: entry.UpdatedAtMs,
	}
	if entry.Progress != nil {
		label := entry.Progress.Label
		rec.ProgressLabel = &label
		rec.ProgressDone = entry.Progress.Done
		rec.ProgressTotal = entry.Progress.Total
	}
	return store.SaveProvider(ctx, rec)
}

func handleListProviders(w http.ResponseWriter, r *http.Request) {
	store, ok := requireConnectorStore(w)
	if !ok {
		return
	}
	entry, err := loadProviderEntry(r.Context(), store, binanceProvider.id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": []providerView{providerViewFor(entry)}})
}

func handleGetProvider(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("id") != binanceProvider.id {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	store, ok := requireConnectorStore(w)
	if !ok {
		return
	}
	entry, err := loadProviderEntry(r.Context(), store, binanceProvider.id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, providerViewFor(entry))
}

func handleDeactivateProvider(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("id") != binanceProvider.id {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	store, ok := requireConnectorStore(w)
	if !ok {
		return
	}
	entry, err := loadProviderEntry(r.Context(), store, binanceProvider.id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	entry.Activated = false
	entry.Status = "inactive"
	entry.Error = ""
	entry.Progress = nil
	entry.UpdatedAtMs = time.Now().UnixMilli()
	if entry.Config == nil {
		entry.Config = map[string]string{}
	}
	if err := saveProviderEntry(r.Context(), store, binanceProvider.id, entry); err != nil {
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
	store, ok := requireConnectorStore(w)
	if !ok {
		return
	}
	entry, err := loadProviderEntry(r.Context(), store, binanceProvider.id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	entry.Status = "setting_up"
	entry.Activated = false
	entry.Error = ""
	zero := 0
	entry.Progress = &providerProgress{Label: "Fetching markets", Done: &zero, Total: nil}
	entry.UpdatedAtMs = time.Now().UnixMilli()
	// The default base URL is shown, not stored: a stored value would hide BINANCE_BASE_URL.
	if err := saveProviderEntry(r.Context(), store, binanceProvider.id, entry); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	go finishProviderSetup(store)
	writeJSON(w, http.StatusAccepted, providerViewFor(entry))
}

// finishProviderSetup fetches the markets and stores the result. A failure stores status failed and
// keeps the previous snapshot; a success stores the snapshot and the state in one transaction.
func finishProviderSetup(store *pgcatalog.Catalog) {
	providerSetupMu.Lock()
	defer providerSetupMu.Unlock()
	ctx := context.Background()
	entry, err := loadProviderEntry(ctx, store, binanceProvider.id)
	if err != nil {
		log.Printf("connectors: load %s: %v", binanceProvider.id, err)
		return
	}
	// A setup always asks the exchange: the in-memory pairs cache must not answer an activation.
	pairs, err := listProviderPairs(ctx, binanceProvider.id, true)
	if err != nil {
		entry.Status = "failed"
		entry.Activated = false
		entry.Error = err.Error()
		entry.UpdatedAtMs = time.Now().UnixMilli()
		if serr := saveProviderEntry(ctx, store, binanceProvider.id, entry); serr != nil {
			log.Printf("connectors: save failed state: %v", serr)
		}
		return
	}
	n := len(pairs)
	entry.Activated = true
	entry.Status = "ready"
	entry.Error = ""
	entry.Progress = &providerProgress{Label: "Loaded pairs", Done: &n, Total: &n}
	entry.UpdatedAtMs = time.Now().UnixMilli()
	rec := pgcatalog.ProviderRecord{
		ID: binanceProvider.id, Activated: entry.Activated, Status: entry.Status, Error: entry.Error,
		Config: entry.Config, UpdatedAtMs: entry.UpdatedAtMs,
		ProgressLabel: &entry.Progress.Label, ProgressDone: entry.Progress.Done, ProgressTotal: entry.Progress.Total,
	}
	snapshot := make([]pgcatalog.ProviderPair, 0, len(pairs))
	for _, p := range pairs {
		snapshot = append(snapshot, pgcatalog.ProviderPair{Base: p.Base, Quote: p.Quote})
	}
	if err := store.SaveProviderSetup(ctx, rec, snapshot); err != nil {
		log.Printf("connectors: save setup: %v", err)
	}
}
