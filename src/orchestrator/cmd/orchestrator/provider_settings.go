package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/MaxBernstetter/dagobert/src/experimentapi/providers"
)

// jobBaseURL is the Binance base URL handed to job containers: the saved config, else BINANCE_BASE_URL, else the
// public default (providers.Service.BaseURL).
func jobBaseURL() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return providerSvc.BaseURL(ctx, providers.BinanceID)
}

type providerPatch struct {
	Config  map[string]string `json:"config"`
	Secrets map[string]string `json:"secrets"`
}

// handlePatchProvider saves the config and the secret values. Deprecated: see mountConnectors.
func handlePatchProvider(w http.ResponseWriter, r *http.Request) {
	var body providerPatch
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	v, err := providerSvc.Save(r.Context(), r.PathValue("id"), body.Config, body.Secrets)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
