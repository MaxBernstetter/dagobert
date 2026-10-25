package provider

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/MaxBernstetter/dagobert/src/shared"
)

// Candle is the provider-normalized 1h bar (same columns as lake parquet).
type Candle struct {
	OpenTime  int64
	Open      string
	High      string
	Low       string
	Close     string
	Volume    string
	CloseTime int64
}

// Pair is a tradeable market the provider can candle (no synthetic crosses).
type Pair struct {
	Base         string // BTC
	Quote        string // USDT
	NativeSymbol string // provider wire id (Binance: BTCUSDT)
}

// PageRequest is one HTTP page for a time window (facade owns windowing).
type PageRequest struct {
	Base  string
	Quote string
	Start int64
	End   int64 // inclusive endTime
}

// PageResult is one FetchPage attempt (no retries).
type PageResult struct {
	Candles    []Candle
	StatusCode int           // 0 if failed before HTTP
	RetryAfter time.Duration // <0 if absent
	Err        error
}

// Adapter fetches candles for one market-data provider.
// Fetch mechanics (semaphore, retry, rate limit, window split) live in the facade.
type Adapter interface {
	ID() string
	DataSource() shared.DataSource
	EmptyReason() string
	MaxPageHours() int
	RequestWeight() int
	Retryable(status int, err error) bool
	ListPairs(ctx context.Context) ([]Pair, error)
	FetchPage(ctx context.Context, req PageRequest) PageResult
}

var (
	regMu    sync.RWMutex
	registry = map[string]Adapter{}
)

// Register adds an adapter. Panics on duplicate id.
func Register(a Adapter) {
	id := a.ID()
	regMu.Lock()
	defer regMu.Unlock()
	if _, ok := registry[id]; ok {
		panic(fmt.Sprintf("provider: duplicate register %q", id))
	}
	registry[id] = a
}

// Get returns a registered adapter by id.
func Get(id string) (Adapter, error) {
	regMu.RLock()
	defer regMu.RUnlock()
	a, ok := registry[id]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", id)
	}
	return a, nil
}

// DefaultID is the ingest provider when none is specified.
func DefaultID() string {
	return shared.DataSourceBinance.String()
}
