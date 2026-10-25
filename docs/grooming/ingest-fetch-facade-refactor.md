# Ingest fetch mechanics → facade

Follow-up to [ingest-provider-facade](ingest-provider-facade.md). Hive layout and Binance as first adapter stay; this slice **re-homes HTTP transport** so adapters only carry provider-specific content.

## Problem

`Adapter.Fetch` is a black box. Binance still owns semaphore, retry/backoff, weight limiting, and window fan-out in `src/ingest/provider/binance/fetch.go`. A second provider would duplicate that stack.

**Rule:** mechanism is facade (configurable). Provider differences are adapter content.

## Target split

```text
job.Run
  → facade fetchclient.FetchAll
       → window split (MaxPageHours)
       → semaphore (concurrency)
       → rate limiter (RequestWeight)
       → retry / backoff (env + Retryable / RetryAfter)
       → Adapter.FetchPage   ← one HTTP attempt
            → route, auth, parse, status policy
```

### Adapter (provider-only)

| Concern | Owner |
|---------|--------|
| Base URL / route / query | Adapter config + request builder |
| Auth | Adapter (Binance: none) |
| Max page size, request weight | Adapter constants (API docs) |
| Retryable statuses / `Retry-After` parse | Adapter policy |
| Body → `[]Candle` | Adapter |
| `ID`, `DataSource`, `EmptyReason` | Adapter |

Proposed seam (replace coarse `Fetch`):

```go
type PageRequest struct {
	Symbol string
	Start  int64
	End    int64 // inclusive endTime; facade owns windowing
}

type PageResult struct {
	Candles    []Candle
	StatusCode int           // 0 if failed before HTTP
	RetryAfter time.Duration // <0 if absent
	Err        error
}

type Adapter interface {
	ID() string
	DataSource() shared.DataSource
	EmptyReason() string
	MaxPageHours() int
	RequestWeight() int
	Retryable(status int, err error) bool
	FetchPage(ctx context.Context, req PageRequest) PageResult
}
```

Binance `FetchPage`: one GET `/api/v3/klines`, map status/`Retry-After`, `parseKlines`. No retry loop, semaphore, or limiter.

### Facade (shared + configurable)

Package: `src/ingest/internal/fetchclient` (used by `job.Run`).

- Window split using `ad.MaxPageHours()` (remove job-hardcoded `pageLimit`).
- Concurrent pages + semaphore from `concurrency` env.
- Weight limiter; each page acquires `ad.RequestWeight()`.
- Retry loop: `ad.Retryable` + `RetryAfter` or exponential backoff from env (`max_retries`, `backoff_factor`, `retry_wait`, `max_retry_wait`).
- `binance.Options` shrinks to **`BaseURL` only**; transport knobs stay on `job.Config` and feed `fetchclient`, not the adapter.

```go
func FetchAll(ctx context.Context, ad provider.Adapter, symbol string, wins [][2]int64, cfg TransportConfig) ([]provider.Candle, error)
```

## File moves

| From | To |
|------|----|
| binance weightLimiter, sem, getPage retry, backoff | `internal/fetchclient` |
| binance `do` + `parseKlines` + `retryable` | stay as `FetchPage` + `Retryable` |
| `job.windows` + `pageLimit` | fetchclient (via `MaxPageHours`) |
| transport fields on `binance.Options` | remove |

## Tests

- `fetchclient`: fake adapter — retryable vs not, `RetryAfter`, concurrency.
- Binance: parse + `Retryable` table (429/418/5xx vs 400).
- System ingest: behaviour unchanged after image rebuild.

## Docs

Update this file’s parent grooming note and `docs/ingest/binance-public-klines.md`: transport env vars = facade; Binance section = route, weight, status semantics only.

## Out of scope

- Second exchange adapter  
- Real auth beyond the seam  
- Hive / sqlite changes  

## Acceptance

- No retry/semaphore/limiter under `src/ingest/provider/binance/`  
- New provider = `FetchPage` + policy methods only  
- Ingest system tests green with default transport env  
- `BINANCE_BASE_URL` is the only Binance connection setting; transport envs stay global  

## Status

Implemented. Unit tests: `./src/ingest/internal/fetchclient`, `./src/ingest/provider/binance`. Re-run system ingest after image rebuild.
