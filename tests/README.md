# tests

`tests/system/<job>/` builds the real job image with `docker build` and runs it against faked upstreams. The tests do not import service packages, so they prove behaviour at the container boundary.

## Layout
- `system/ingest`, `calculation-technical-indicator`, `calculation-signal`, `execution`: one job each. Fakes: Binance over local HTTP, a seeded Parquet file or SQLite row, a fake Bybit response.
- `system/backtest`: the single-backtest image; skipped without `DATA_DIR` and lake data.
- `system/experiment`: the experiment path (`dagobert-backtest`), on a scratch Postgres (`POSTGRES_TEST_URL`; the tests fail without it): engine switch and C++ (`engine_test.go`), persistence (`persist_test.go`, `persist_pg_test.go`), ma-cross sizing, the API over the instance Parquet files (`api_instance_files_test.go`), and a persistence benchmark (`bench_persist_test.go`, needs `BENCH_INSTANCES`). Fixtures are created by `createExperimentPG` (`pg_fixture_test.go`).
- `system/internal/lakefile`: shared helper that writes seeded candle files.
- `bruno/binance`: manual HTTP requests, not run by CI.

## Run
```bash
task test                                                   # all system tests, about 20 min
go test ./tests/system/ingest -run TestName -count=1 -timeout 20m   # one test, image built by the test
go test ./src/internal/... ./src/shared/...                 # unit tests, shared packages
go test ./src/backtest/...                                  # unit and equivalence tests
task cpp:test                                               # C++ tests (ctest), PRESET=debug|optimized
```
Docker is required. Image builds dominate the duration; a single test takes minutes.

## Skips
- Postgres tests skip when no Postgres is reachable (start it with `task db:up`).
- C++ tests skip when the strategy binary is missing: run `task cpp:build`, or point `CPP_SMA_CROSS_BIN` / `CPP_SIGNAL_CONSENSUS_BIN` at a binary.
- Some tests skip when docker or a data dir is missing.

## Before changing
Do not add tests that lock internal call order or private types (`docs/development/04-coding-principles.md`). Unit tests are for pure logic only.
