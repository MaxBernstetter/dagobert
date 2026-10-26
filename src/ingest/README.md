# ingest

First job in the pipeline: fetches Binance public klines and writes Hive-partitioned Parquet under `data/parquet/` plus `ingest.sqlite`. Layout: `cmd/ingest`, `internal/job` (work), `internal/fetchclient` (HTTP), `provider/` (`provider.go` interface, `binance/`), `runner/` (container contract).

Missing exchange hours are recorded as `unavailable`, not synthesized; coverage checks treat only retryable gaps as blocking. A close that is already stored exits 2 (no-op). Backtests reuse this package in-process to fill gaps. Candle schema: `docs/ingest/binance-public-klines.md`. Test: `go test ./tests/system/ingest -count=1` (fake Binance, needs docker).
