package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/MaxBernstetter/dagobert/src/experimentapi/providers"
)

// providerSvc serves the provider settings. The Connect ProviderService (mounted with the Experiment API) and
// the REST routes below share it; serveBackfill replaces it with the one of the Experiment API server.
var providerSvc = newProviderService(nil)

// newProviderService builds the provider settings service over store (nil without DATABASE_URL) and the secret
// backend of the environment.
func newProviderService(store providers.Store) *providers.Service {
	secrets, err := providers.SecretsFromEnv(context.Background())
	if err != nil {
		log.Printf("connectors: secrets: %v", err)
		secrets = providers.Unavailable(err)
	}
	return providers.NewService(store, secrets)
}

// mountConnectors mounts the provider REST routes.
//
// Deprecated: the Connectors page uses ProviderService (Connect, src/experimentapi/provider_service.go). These
// routes are kept for existing clients and are removed in a follow-up. They delegate to the same service, so the
// behaviour is one implementation.
func mountConnectors(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/providers", handleListProviders)
	mux.HandleFunc("GET /api/v1/providers/{id}", handleGetProvider)
	mux.HandleFunc("PATCH /api/v1/providers/{id}", handlePatchProvider)
	mux.HandleFunc("POST /api/v1/providers/{id}/activate", handleActivateProvider)
	mux.HandleFunc("POST /api/v1/providers/{id}/deactivate", handleDeactivateProvider)
}

// writeProviderError maps a providers error to the REST status: 404 unknown provider, 503 without DATABASE_URL,
// 400 rejected key, 502 exchange unreachable, else 500.
func writeProviderError(w http.ResponseWriter, err error) {
	var verr providers.ValidationError
	switch {
	case errors.Is(err, providers.ErrUnknownProvider):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	case errors.Is(err, providers.ErrStoreRequired):
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
	case errors.As(err, &verr):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": verr.Msg})
	case errors.Is(err, providers.ErrUpstream):
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
}

func handleListProviders(w http.ResponseWriter, r *http.Request) {
	views, err := providerSvc.List(r.Context())
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": views})
}

func handleGetProvider(w http.ResponseWriter, r *http.Request) {
	v, err := providerSvc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func handleDeactivateProvider(w http.ResponseWriter, r *http.Request) {
	v, err := providerSvc.Deactivate(r.Context(), r.PathValue("id"))
	if err != nil {
		writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// handleActivateProvider answers 202 at once with setting_up; the setup runs in the background and the client
// polls GET. (ProviderService.ActivateProvider runs the setup before it answers.)
func handleActivateProvider(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	v, err := providerSvc.StartSetup(r.Context(), id)
	if err != nil {
		writeProviderError(w, err)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := providerSvc.RunSetup(ctx, id); err != nil {
			log.Printf("connectors: setup %s: %v", id, err)
		}
	}()
	writeJSON(w, http.StatusAccepted, v)
}
