package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/MaxBernstetter/dagobert/src/experimentapi/providers"
)

// handleProviderPairs serves the market snapshot of a ready provider, else the live markets (nothing stored).
// Deprecated: ProviderService.ListProviderPairs serves the same data; see mountConnectors.
func handleProviderPairs(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.Error(w, "provider id required", http.StatusBadRequest)
		return
	}
	pairs, err := providerSvc.Pairs(r.Context(), id)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	if pairs == nil {
		pairs = []providers.Pair{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, s-maxage=86400, stale-while-revalidate=86400")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"provider": id,
		"pairs":    pairs,
	})
}
