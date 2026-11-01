package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/MaxBernstetter/dagobert/src/ingest/provider"
	"github.com/MaxBernstetter/dagobert/src/ingest/provider/binance"
)

type pairsCacheEntry struct {
	at    time.Time
	pairs []provider.Pair
}

var (
	pairsCacheMu sync.Mutex
	pairsCache   = map[string]pairsCacheEntry{}
)

const pairsCacheTTL = 24 * time.Hour

// handleProviderPairs serves the market snapshot stored by the provider's setup. A provider that is not
// ready has no snapshot yet; then the markets are fetched live, as before, and nothing is stored.
func handleProviderPairs(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.Error(w, "provider id required", http.StatusBadRequest)
		return
	}
	store, ok := requireConnectorStore(w)
	if !ok {
		return
	}
	rec, found, err := store.GetProvider(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type row struct {
		Base  string `json:"base"`
		Quote string `json:"quote"`
	}
	var out []row
	if found && rec.Status == "ready" {
		stored, err := store.ProviderPairs(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		out = make([]row, 0, len(stored))
		for _, p := range stored {
			out = append(out, row{Base: p.Base, Quote: p.Quote})
		}
	} else {
		pairs, err := listProviderPairs(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		out = make([]row, 0, len(pairs))
		for _, p := range pairs {
			out = append(out, row{Base: p.Base, Quote: p.Quote})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, s-maxage=86400, stale-while-revalidate=86400")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"provider": id,
		"pairs":    out,
	})
}

func listProviderPairs(ctx context.Context, id string) ([]provider.Pair, error) {
	pairsCacheMu.Lock()
	if e, ok := pairsCache[id]; ok && time.Since(e.at) < pairsCacheTTL {
		pairsCacheMu.Unlock()
		return e.pairs, nil
	}
	pairsCacheMu.Unlock()

	ad, err := openProviderAdapter(id)
	if err != nil {
		return nil, err
	}
	pairs, err := ad.ListPairs(ctx)
	if err != nil {
		return nil, err
	}
	pairsCacheMu.Lock()
	pairsCache[id] = pairsCacheEntry{at: time.Now(), pairs: pairs}
	pairsCacheMu.Unlock()
	return pairs, nil
}

func openProviderAdapter(id string) (provider.Adapter, error) {
	switch id {
	case "binance":
		base := strings.TrimSpace(os.Getenv("BINANCE_BASE_URL"))
		if base == "" {
			base = "https://data-api.binance.vision"
		}
		return binance.New(binance.Options{BaseURL: base})
	default:
		return nil, fmt.Errorf("unknown provider %s", id)
	}
}
