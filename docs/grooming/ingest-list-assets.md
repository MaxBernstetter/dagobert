# Facade: list pairs per provider + currency-normalized lake

## Locked decisions

| # | Choice |
|---|--------|
| Provider | **Binance** (catalog + candles) |
| UI flow | Pick **provider** → load that provider’s pairs → fill the market control |
| List cache | Cache the list (pairs change rarely); cache key = provider id |
| Disk layout | **`provider / base / quote`** folders (not a single `pair=` folder) |
| Saved settings | Store **`base` and `quote` as two fields** (not one formatted string like `BTC-USDT`) |
| How many pairs | **All** tradeable spot pairs Binance reports; drop impossible ones (e.g. **base = quote**); UI stays usable via **search** + clear row layout |
| Friendly names (“Bitcoin”) | Official Spot docs (`exchangeInfo`) only expose **codes** (`baseAsset` / `quoteAsset`), not full names. **Skip names for now**; show tickers only |
| Existing lake data | **Do not migrate** — wipe already loaded candle/index data when cutting over |

Ingest still **never invents** pairs: only write candles for markets the provider actually lists.

## Why base + quote on disk and in settings

A market is “this currency priced in that currency.” Storing the two sides separately keeps the model honest (any base vs any quote the exchange offers: `BTC`/`ETH`, `XRP`/`EUR`, `ETH`/`USD`, …). Formatting into `BTC-USDT` is a display/API convenience only, not the source of truth.

## Problem today

Hardcoded `BTCUSDT`-style symbols in UI, ingest allowlists, and Hive `symbol=BTCUSDT`. No discovery of what Binance can ingest; opaque concat encoding.

## Target model

| Concept | Meaning |
|---------|---------|
| **Currency** | Code such as `BTC`, `USDT`, `EUR` |
| **Pair / market** | `(base, quote)` — price of one unit of base in quote |
| **Provider product** | Exchange’s own id (Binance `BTCUSDT`) — adapter-only when talking to the API |

### On disk (Hive)

```text
DATA_DIR/parquet/
  provider=binance/
    base=BTC/
      quote=USDT/
        year=2024/
          candles.parquet   # footer KV metadata (no sidecar)
```

### sqlite

`stored` / `unavailable` keyed by `(provider, base, quote, open_time)` (replace `symbol`).

### Backtest / experiment settings

```json
{ "provider": "binance", "base": "BTC", "quote": "USDT", ... }
```

Drop opaque `symbol` once cut over (or keep read-compat only during the wipe window — prefer clean break with dump).

### Cutover / wipe

Document a one-time wipe of local lake artifacts (e.g. `parquet/`, `ingest.sqlite`, and anything that assumed old `symbol=` paths). No automated migrate-from-`BTCUSDT`.

## Binance adapter

```go
type Pair struct {
	Base         string // BTC
	Quote        string // USDT
	NativeSymbol string // BTCUSDT — wire id for klines only
}

type Adapter interface {
	// existing FetchPage + policy ...
	ListPairs(ctx context.Context) ([]Pair, error)
}
```

`ListPairs`:

1. `GET /api/v3/exchangeInfo` (Spot docs).
2. Keep spot / `TRADING` (and whatever filter matches “ready for 1h klines”).
3. Map each row → `Base`, `Quote`, `NativeSymbol`.
4. **Exclude** `base == quote`.
5. Sort by base, then quote (case-insensitive).
6. No full-name field until a **documented** Binance source exists.

`FetchPage`: job passes `base`+`quote`; adapter resolves `NativeSymbol` (from ListPairs cache or Binance concat rules) and calls klines. If the pair is not on the exchange, fail — do not synthesize.

Relax ingest allowlists: accept any validated base/quote partition values the adapter accepts for that run.

## API

```http
GET /api/v1/providers/{id}/pairs
→ {
  "provider": "binance",
  "pairs": [
    { "base": "BTC", "quote": "USDT" },
    ...
  ]
}
```

Orchestrator (or thin catalog entrypoint) calls the Binance adapter; Next route proxies and caches:

- `Cache-Control: public, s-maxage=3600, stale-while-revalidate=86400` (tune as needed)
- or `fetch(..., { next: { revalidate: 3600 } })` keyed by provider

## UI

1. Provider select (unchanged idea).
2. Market combobox: loads pairs for that provider; **search** over base and quote; alphabetical; each row shows **base** and **quote** clearly (e.g. `BTC` · `USDT`) so many pairs stay scannable without full names.
3. Changing provider reloads pairs and clears an invalid selection.
4. Experiment GRID market axis uses the same list.
5. Persist `base` + `quote` on create.

Avoid overload: virtualized or capped visible list inside the popover is fine; search is required; do not dump hundreds of `<option>`s in a raw `<select>`.

## Out of scope

- Friendly names (“Bitcoin”) until documented API exists  
- Synthetic crosses at ingest  
- Other exchanges  
- Migrating old `symbol=BTCUSDT` trees  

## Acceptance

- Hive paths use `provider` / `base` / `quote`; sqlite matches.  
- Binance `ListPairs` returns all eligible spot markets minus base=quote; FetchPage uses native symbol only for real products.  
- Next caches `GET …/pairs` per provider.  
- Backtest + experiment: provider → searchable market control → stores base+quote.  
- Cutover docs say wipe old data; no migrate path required.  

## Implementation order

1. Shared path helpers + sqlite PK for base/quote; wipe note in run docs.  
2. Binance `ListPairs` + resolve native symbol for FetchPage; drop hardcoded symbol allowlists.  
3. Orchestrator pairs endpoint + Next cached proxy.  
4. Searchable market combobox on backtest + experiment; settings use base+quote.  
5. Tests (exchangeInfo fixture, paths, filter base=quote) + docs.

## Status

Implemented (cutover: wipe `data/parquet` + `data/ingest.sqlite`).  
