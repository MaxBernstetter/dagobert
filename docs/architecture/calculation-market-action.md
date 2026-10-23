# Calculation: market action (layer 2)

Layer 2 sits between **strategy** (signals) and **execution** (venue orders).

## Inputs

| Input | Source |
|-------|--------|
| Signal | `calculation_signal.sqlite` → `signal` for `(playbook, symbol, candle_close)` |
| Cash & base | **Live Bybit** `GET /v5/account/wallet-balance?accountType=UNIFIED&coin=…` ([docs](https://bybit-exchange.github.io/docs/v5/account/wallet-balance)); on `api-demo.bybit.eu` HTTP 404, falls back to transaction-log |
| Mark price | SMA(3h) from `technical_indicator.sqlite` at this bar (quote per base, Binance-derived) |

Execution’s cached `execution.sqlite` balances are **not** used for sizing. That removed the drift where the UI snapshot lagged the venue.

## Outputs

| Output | Store |
|--------|--------|
| Sized decision | `calculation_signal.sqlite` → `action` (`decision`, `size`, `rule`, …) |

Idempotency: duplicate `action` row → exit `2` / detail `1`.

## Allocation policy (defaults)

| Parameter | Default | Meaning |
|-----------|---------|---------|
| `ALLOC_TARGET_WEIGHT_PCT` | `10` | Cap exposure at 10% of portfolio value |
| (fixed) reserve | 50 USDC | Never spend below this cash |
| (fixed) min order | 1 USDC | Bybit demo minimum |

**Portfolio value** = quote cash + base × mark.

**Buy signal, flat exposure below target:** spend `min(room_to_target, cash − reserve)`; if below min → `no-trade` with rule note.

**Sell signal:** sell **all** base held at the venue for that instrument.

**Signal `no-trade`:** write `no-trade` action (detail `2` on success).

## Pipeline

`ingest` → `calculation-technical-indicator` → `strategy` → `calculation-market-action` → `execution`
