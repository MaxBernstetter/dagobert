// Package providers holds the ingestion provider settings: the declared fields, the activation state in
// Postgres, the secrets (SecretStore) and the setup against the exchange. The ProviderService (Connect) and
// the orchestrator's REST routes both call it, so the logic lives once.
package providers

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/MaxBernstetter/dagobert/src/experimentapi/pgcatalog"
	"github.com/MaxBernstetter/dagobert/src/ingest/provider"
	"github.com/MaxBernstetter/dagobert/src/ingest/provider/binance"
)

// BinanceID is the only plugged-in provider.
const BinanceID = "binance"

// DefaultBinanceBaseURL is the public Binance data host, used when neither the config nor the environment set one.
const DefaultBinanceBaseURL = "https://data-api.binance.vision"

const pairsCacheTTL = 24 * time.Hour

var (
	// ErrUnknownProvider is returned for an id that is not plugged in.
	ErrUnknownProvider = errors.New("not found")
	// ErrStoreRequired is returned when no Postgres catalog is configured (DATABASE_URL).
	ErrStoreRequired = errors.New("DATABASE_URL required: connector state is stored in Postgres")
	// ErrUpstream wraps a failure to reach the exchange for a live listing.
	ErrUpstream = errors.New("provider upstream")
)

// ValidationError is a rejected config or secret key.
type ValidationError struct{ Msg string }

func (e ValidationError) Error() string { return e.Msg }

// Store is the connector state in Postgres. *pgcatalog.Catalog implements it.
type Store interface {
	GetProvider(ctx context.Context, id string) (pgcatalog.ProviderRecord, bool, error)
	SaveProvider(ctx context.Context, rec pgcatalog.ProviderRecord) error
	ProviderPairs(ctx context.Context, id string) ([]pgcatalog.ProviderPair, error)
	SaveProviderSetup(ctx context.Context, rec pgcatalog.ProviderRecord, pairs []pgcatalog.ProviderPair) error
}

// Field is one declared config or secret key of a provider. Kind "secret" marks a write-only value.
type Field struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Kind        string `json:"kind"`
	Description string `json:"description,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Default     string `json:"default,omitempty"`
}

// Progress is the setup progress: Done and Total are nil when unknown.
type Progress struct {
	Label string `json:"label"`
	Done  *int   `json:"done"`
	Total *int   `json:"total"`
}

// View is one provider with its stored state. Secret values are never in a View.
type View struct {
	ID                   string            `json:"id"`
	Label                string            `json:"label"`
	Description          string            `json:"description,omitempty"`
	Fields               []Field           `json:"fields"`
	Activated            bool              `json:"activated"`
	Status               string            `json:"status"`
	Error                string            `json:"error"`
	Progress             *Progress         `json:"progress"`
	Config               map[string]string `json:"config"`
	SecretKeysConfigured []string          `json:"secretKeysConfigured"`
	UpdatedAtMs          int64             `json:"updatedAtMs"`
}

// Pair is one market of a provider.
type Pair struct {
	Base  string `json:"base"`
	Quote string `json:"quote"`
}

// BinanceFields is the declared field set of Binance.
func BinanceFields() []Field {
	return []Field{{
		Key:         "baseUrl",
		Label:       "Base URL",
		Kind:        "string",
		Description: "Public data API host for klines and exchangeInfo.",
		Placeholder: DefaultBinanceBaseURL,
		Default:     DefaultBinanceBaseURL,
	}}
}

// state is the stored state of one provider, as the service reads and writes it.
type state struct {
	Activated   bool
	Status      string
	Error       string
	Progress    *Progress
	Config      map[string]string
	UpdatedAtMs int64
}

type pairsEntry struct {
	at    time.Time
	pairs []provider.Pair
}

// Service serves the provider settings. Setup runs are serialised; listings are cached in memory for 24 h.
type Service struct {
	store   Store
	secrets SecretStore
	fields  []Field

	setupMu sync.Mutex
	pairsMu sync.Mutex
	pairs   map[string]pairsEntry
}

// NewService serves the Binance provider. A nil store answers ErrStoreRequired on every state call.
func NewService(store Store, secrets SecretStore) *Service {
	return NewServiceWithFields(store, secrets, BinanceFields())
}

// NewServiceWithFields is NewService with an explicit declared field set (tests declare a secret field).
func NewServiceWithFields(store Store, secrets SecretStore, fields []Field) *Service {
	return &Service{store: store, secrets: secrets, fields: fields, pairs: map[string]pairsEntry{}}
}

// List returns the plugged-in providers with their state.
func (s *Service) List(ctx context.Context) ([]View, error) {
	v, err := s.Get(ctx, BinanceID)
	if err != nil {
		return nil, err
	}
	return []View{v}, nil
}

// Get returns one provider with its state.
func (s *Service) Get(ctx context.Context, id string) (View, error) {
	if err := s.require(id); err != nil {
		return View{}, err
	}
	st, err := s.load(ctx, id)
	if err != nil {
		return View{}, err
	}
	return s.view(ctx, id, st)
}

// Save stores declared config keys and secret values. Activation is not changed: a new base URL takes effect
// when the provider is activated again.
func (s *Service) Save(ctx context.Context, id string, config, secrets map[string]string) (View, error) {
	if err := s.require(id); err != nil {
		return View{}, err
	}
	for _, key := range slices.Sorted(maps.Keys(config)) {
		if f, ok := s.field(key); !ok || f.Kind == "secret" {
			return View{}, ValidationError{Msg: "unknown config key " + key}
		}
	}
	for _, key := range slices.Sorted(maps.Keys(secrets)) {
		if f, ok := s.field(key); !ok || f.Kind != "secret" {
			return View{}, ValidationError{Msg: "unknown secret key " + key}
		}
	}
	st, err := s.load(ctx, id)
	if err != nil {
		return View{}, err
	}
	merged := map[string]string{}
	maps.Copy(merged, st.Config)
	for k, v := range config {
		merged[k] = strings.TrimSpace(v)
	}
	st.Config = merged
	if len(secrets) > 0 {
		if err := s.secrets.Put(ctx, id, secrets); err != nil {
			return View{}, err
		}
	}
	st.UpdatedAtMs = time.Now().UnixMilli()
	if err := s.save(ctx, id, st); err != nil {
		return View{}, err
	}
	return s.view(ctx, id, st)
}

// StartSetup marks the provider as setting up and returns that view. RunSetup does the work.
func (s *Service) StartSetup(ctx context.Context, id string) (View, error) {
	if err := s.require(id); err != nil {
		return View{}, err
	}
	st, err := s.load(ctx, id)
	if err != nil {
		return View{}, err
	}
	zero := 0
	st.Status = "setting_up"
	st.Activated = false
	st.Error = ""
	st.Progress = &Progress{Label: "Fetching markets", Done: &zero}
	st.UpdatedAtMs = time.Now().UnixMilli()
	if err := s.save(ctx, id, st); err != nil {
		return View{}, err
	}
	return s.view(ctx, id, st)
}

// RunSetup fetches the markets from the exchange and stores the result. A failed setup is stored as status
// failed (the previous snapshot is kept) and is not an error here; the returned error is a storage error only.
func (s *Service) RunSetup(ctx context.Context, id string) error {
	s.setupMu.Lock()
	defer s.setupMu.Unlock()
	st, err := s.load(ctx, id)
	if err != nil {
		return err
	}
	// A setup always asks the exchange: the cached listing must not answer an activation.
	pairs, err := s.listPairs(ctx, id, true)
	if err != nil {
		st.Status = "failed"
		st.Activated = false
		st.Error = err.Error()
		st.UpdatedAtMs = time.Now().UnixMilli()
		return s.save(ctx, id, st)
	}
	n := len(pairs)
	st.Activated = true
	st.Status = "ready"
	st.Error = ""
	st.Progress = &Progress{Label: "Loaded pairs", Done: &n, Total: &n}
	st.UpdatedAtMs = time.Now().UnixMilli()
	snapshot := make([]pgcatalog.ProviderPair, 0, len(pairs))
	for _, p := range pairs {
		snapshot = append(snapshot, pgcatalog.ProviderPair{Base: p.Base, Quote: p.Quote})
	}
	return s.store.SaveProviderSetup(ctx, toRecord(id, st), snapshot)
}

// Activate runs the setup before it returns: the view is ready or failed.
func (s *Service) Activate(ctx context.Context, id string) (View, error) {
	if _, err := s.StartSetup(ctx, id); err != nil {
		return View{}, err
	}
	if err := s.RunSetup(ctx, id); err != nil {
		return View{}, err
	}
	return s.Get(ctx, id)
}

// Deactivate drops the provider from the ready set. The snapshot and the config are kept.
func (s *Service) Deactivate(ctx context.Context, id string) (View, error) {
	if err := s.require(id); err != nil {
		return View{}, err
	}
	st, err := s.load(ctx, id)
	if err != nil {
		return View{}, err
	}
	st.Activated = false
	st.Status = "inactive"
	st.Error = ""
	st.Progress = nil
	st.UpdatedAtMs = time.Now().UnixMilli()
	if st.Config == nil {
		st.Config = map[string]string{}
	}
	if err := s.save(ctx, id, st); err != nil {
		return View{}, err
	}
	return s.view(ctx, id, st)
}

// Pairs returns the stored snapshot when the provider is ready, otherwise the markets fetched live (nothing is
// stored). A live failure wraps ErrUpstream.
func (s *Service) Pairs(ctx context.Context, id string) ([]Pair, error) {
	if err := s.require(id); err != nil {
		return nil, err
	}
	rec, found, err := s.store.GetProvider(ctx, id)
	if err != nil {
		return nil, err
	}
	var out []Pair
	if found && rec.Status == "ready" {
		stored, err := s.store.ProviderPairs(ctx, id)
		if err != nil {
			return nil, err
		}
		for _, p := range stored {
			out = append(out, Pair{Base: p.Base, Quote: p.Quote})
		}
		return out, nil
	}
	live, err := s.listPairs(ctx, id, false)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	for _, p := range live {
		out = append(out, Pair{Base: p.Base, Quote: p.Quote})
	}
	return out, nil
}

// BaseURL is the effective base URL of a provider: the saved config baseUrl, else BINANCE_BASE_URL, else the
// public default. Without a readable store it falls back to the environment and the default.
func (s *Service) BaseURL(ctx context.Context, id string) string {
	envValue := os.Getenv("BINANCE_BASE_URL")
	if s.store == nil {
		return EffectiveBaseURL("", envValue)
	}
	rec, ok, err := s.store.GetProvider(ctx, id)
	if err != nil || !ok {
		return EffectiveBaseURL("", envValue)
	}
	return EffectiveBaseURL(rec.Config["baseUrl"], envValue)
}

// EffectiveBaseURL is the base URL a provider reads from: the saved config baseUrl when set, else the
// BINANCE_BASE_URL environment value, else the public default.
func EffectiveBaseURL(saved, envValue string) string {
	if v := strings.TrimSpace(saved); v != "" {
		return v
	}
	if v := strings.TrimSpace(envValue); v != "" {
		return v
	}
	return DefaultBinanceBaseURL
}

// listPairs returns the markets of a provider from its effective base URL. fresh bypasses the cache; the
// result is cached either way.
func (s *Service) listPairs(ctx context.Context, id string, fresh bool) ([]provider.Pair, error) {
	base := s.BaseURL(ctx, id)
	key := id + "|" + base
	if !fresh {
		s.pairsMu.Lock()
		if e, ok := s.pairs[key]; ok && time.Since(e.at) < pairsCacheTTL {
			s.pairsMu.Unlock()
			return e.pairs, nil
		}
		s.pairsMu.Unlock()
	}
	ad, err := openAdapter(id, base)
	if err != nil {
		return nil, err
	}
	pairs, err := ad.ListPairs(ctx)
	if err != nil {
		return nil, err
	}
	s.pairsMu.Lock()
	s.pairs[key] = pairsEntry{at: time.Now(), pairs: pairs}
	s.pairsMu.Unlock()
	return pairs, nil
}

func openAdapter(id, base string) (provider.Adapter, error) {
	switch id {
	case BinanceID:
		return binance.New(binance.Options{BaseURL: base})
	default:
		return nil, fmt.Errorf("unknown provider %s", id)
	}
}

func (s *Service) require(id string) error {
	if id != BinanceID {
		return ErrUnknownProvider
	}
	if s.store == nil {
		return ErrStoreRequired
	}
	return nil
}

func (s *Service) field(key string) (Field, bool) {
	for _, f := range s.fields {
		if f.Key == key {
			return f, true
		}
	}
	return Field{}, false
}

func (s *Service) load(ctx context.Context, id string) (state, error) {
	rec, ok, err := s.store.GetProvider(ctx, id)
	if err != nil || !ok {
		return state{}, err
	}
	st := state{
		Activated:   rec.Activated,
		Status:      rec.Status,
		Error:       rec.Error,
		Config:      rec.Config,
		UpdatedAtMs: rec.UpdatedAtMs,
	}
	if rec.ProgressLabel != nil {
		st.Progress = &Progress{Label: *rec.ProgressLabel, Done: rec.ProgressDone, Total: rec.ProgressTotal}
	}
	return st, nil
}

func (s *Service) save(ctx context.Context, id string, st state) error {
	return s.store.SaveProvider(ctx, toRecord(id, st))
}

func toRecord(id string, st state) pgcatalog.ProviderRecord {
	rec := pgcatalog.ProviderRecord{
		ID:          id,
		Activated:   st.Activated,
		Status:      st.Status,
		Error:       st.Error,
		Config:      st.Config,
		UpdatedAtMs: st.UpdatedAtMs,
	}
	if st.Progress != nil {
		label := st.Progress.Label
		rec.ProgressLabel = &label
		rec.ProgressDone = st.Progress.Done
		rec.ProgressTotal = st.Progress.Total
	}
	return rec
}

// view builds the View of a state. A never-saved provider shows the default base URL, not a stored value: a
// stored value would hide BINANCE_BASE_URL.
func (s *Service) view(ctx context.Context, id string, st state) (View, error) {
	cfg := st.Config
	if cfg == nil {
		cfg = map[string]string{}
		for _, f := range s.fields {
			if f.Key == "baseUrl" {
				cfg["baseUrl"] = f.Default
			}
		}
	}
	status := st.Status
	if status == "" {
		status = "inactive"
	}
	values, err := s.secrets.Get(ctx, id)
	if err != nil {
		return View{}, err
	}
	return View{
		ID:                   BinanceID,
		Label:                "Binance",
		Description:          "Public spot klines (no API key).",
		Fields:               s.fields,
		Activated:            st.Activated,
		Status:               status,
		Error:                st.Error,
		Progress:             st.Progress,
		Config:               cfg,
		SecretKeysConfigured: ConfiguredKeys(values),
		UpdatedAtMs:          st.UpdatedAtMs,
	}, nil
}
