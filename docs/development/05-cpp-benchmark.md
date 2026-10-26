# C++ vs Go benchmark (CALC.25)

Issue #110. Phase 1 recorded a baseline and profile; phase 2 (section
"Phase 2: optimisation") applies the cheap fixes and re-measures. The
baseline sections below are kept as recorded.

## Run it

```bash
task cpp:deps PRESET=optimized        # once
task cpp:build PRESET=optimized
task cpp:bench RUNS=15                # or: scripts/cpp-bench.sh 15
```

The script generates a seeded lake (`go run ./src/calculation/testdata/genlakefixture -bench <dir>`:
5840 3h BTC/USDT bars = 730 days from 2024-01-01, geometric random walk, seed 110,
decimal columns as in the real lake since DATA.01; `-legacy` writes the old string schema), runs each optimized strategy
executable `RUNS` times, runs the Go benchmark `RUNS` times, and writes
`build/bench/results.json` and `build/bench/results.md`. Configs: sma-cross
fast 12h / slow 48h; signal-consensus defaults.

Profile (Callgrind, optimized build, same lake):

```bash
valgrind --tool=callgrind --callgrind-out-file=build/bench/callgrind.out \
  build/optimized/strategies/sma-cross/strategy-sma-cross \
  --config build/bench/sma-cross.json --data-dir build/bench/lake > /dev/null
callgrind_annotate build/bench/callgrind.out | head -40
```

## What each side measures

| Stage | C++ executable (one process) | Go `BenchmarkGoExperiment` (in process) |
| --- | --- | --- |
| Parquet read | yes (Arrow) | yes (`prepare`, parquet-go) |
| Aggregation + indicators | inline in `calc_backtest` | `prepare`: all deployed indicators for every bar, JSON encoded, KPI window decoded |
| Strategy evaluation | yes | `simulate` (`FastSimulatedPipelineOpts`, slim) |
| Sizing / paper fills | yes (in-memory) | `simulate`: market-action plan, sim venue, SQLite flush |
| Instance report | protobuf serialised to /dev/null | `BuildAndWriteInstanceReport` (report.json, KPIs) |
| Process start | included (fork/exec, dynamic libs) | not included (test binary already running) |
| Postgres catalog, S3, coverage check | no | no (skipped) |

Go `end_to_end` = `prepare` + `simulate` is the experiment path of
`RunExperimentPrepare` + `RunExperimentSimulate` minus catalog/coverage. The
stages are not identical: Go computes and JSON-roundtrips every indicator, and
writes SQLite and a report; C++ computes only what the strategy needs and
keeps everything in memory. The ratio is therefore "experiment path vs
standalone executable", not a pure language comparison. The strategy-only
split is in the `prepare`/`simulate` columns (Go simulate alone is 109 to 261 ms
against 6 to 7 ms for the entire C++ process).

Wall time is per-iteration (C++: fork to wait4; Go: timed region, setup
excluded), median and p95 over `RUNS`. Peak RSS: C++ `ru_maxrss` of the child;
Go `ru_maxrss` of the whole benchmark process (includes the Go runtime and test
harness, and covers the warm-up iteration too).

## Results (this machine)

Intel Core Ultra 9 185H, 22 logical CPUs, Linux 7.0, clang 22.1.8 (C++), Go 1.24.
C++ flags (optimized preset): `-O3 -DNDEBUG -flto=thin -ffunction-sections
-fdata-sections -g -fno-omit-frame-pointer`, lld with `--gc-sections`.
`-march=native` is requested but ignored under Nix (`NIX_ENFORCE_NO_NATIVE`),
so the binaries target generic x86-64. 30 runs each, warm page cache.

| strategy | C++ median ms | C++ p95 ms | C++ RSS MB | Go e2e median ms | Go e2e p95 ms | Go e2e RSS MB | Go prepare ms | Go simulate ms | speedup (median) |
|---|---|---|---|---|---|---|---|---|---|
| sma-cross | 6.4 | 8.3 | 13.3 | 328 | 346 | 81.6 | 210 | 112 | 51x |
| signal-consensus | 6.8 | 8.7 | 13.7 | 499 | 569 | 80.1 | 212 | 261 | 74x |

Go allocations per end-to-end run: 176 MB and 1.6 M allocs (sma-cross), 229 MB
and 2.9 M allocs (consensus).

## Callgrind hotspots (optimized build, ~45 M instructions per run)

Same profile for both strategies (consensus differs by about 5%):

| Share of Ir | Where |
| --- | --- |
| ~50% | `strtod` family in glibc (`__strtod_l_internal`, `str_to_mpn`, `round_and_return`, `__mpn_lshift`, ...), called from `load_bars_from_lake` parsing the string price columns (5 per bar) |
| ~6% | `memcpy` / `__memcpy_chk` (bar loading, string copies) |
| ~5% | Arrow `BinaryBuilder::Append`, `DeltaLengthByteArrayDecoder` (Parquet read) |
| ~5% | `malloc` / `free` |
| ~3% | strategy and signal evaluation (`ConsensusSignal::evaluate`, SMA passes) |

Bar loading dominates; indicator and strategy math is a few percent of a
6 ms run.

## Phase 2: optimisation

Changes, all in `src/calculation/data/bars_parquet.cpp` (`scripts/cpp-bench.sh`
also needed `schemaVersion` 2 in the generated configs, the executables
reject 1):

1. `std::stod` replaced by `std::from_chars` (double). Same contract as
   `stod`: leading whitespace and `+` skipped, trailing characters ignored,
   no digits throws `invalid_argument`, overflow throws `out_of_range`.
2. String columns read through `GetView` (no per-value `std::string`), bars
   moved instead of copied, per-year parts moved into the result.
3. `reader->set_use_threads(false)`: the file is one small row group, and the
   baseline run created an Arrow thread (clone3 in `strace`).

Correctness: `task cpp:test` passes unchanged (91 tests, the suite has 91 in
this tree). The binary `RunReport` on stdout of both executables on the bench
lake is byte-identical before and after (sha256 `b39091c7...` for sma-cross,
`5a577dac...` for signal-consensus, taken from the baseline build).

Results, same lake and seeds, 30 runs each, load average about 1 at the start
of each final run (`uptime`). Machine load matters: intermediate runs below
were taken while load was 2 to 8 right after builds, so use only the first and
last row for conclusions.

| step | sma-cross median / p95 ms | consensus median / p95 ms |
|---|---|---|
| baseline | 6.5 / 8.3 | 7.4 / 8.8 |
| 1 from_chars (load 8, noisy) | 5.7 / 6.5 | 6.4 / 7.9 |
| 1+2 GetView, moves | 5.9 / 6.9 | 6.0 / 6.9 |
| 1+2+3 no thread pool (load 6, noisy) | 5.4 / 6.4 | 6.0 / 7.1 |
| final re-run | 6.0 / 6.6 | 6.3 / 7.0 |

Final vs baseline, same session:

| strategy | C++ median ms | C++ p95 ms | C++ RSS MB | Go e2e median ms | Go e2e p95 ms |
|---|---|---|---|---|---|
| sma-cross before | 6.5 | 8.3 | 13.0 | 407 | 475 |
| sma-cross after | 6.0 | 6.6 | 13.1 | 319 | 360 |
| signal-consensus before | 7.4 | 8.8 | 13.8 | 511 | 610 |
| signal-consensus after | 6.3 | 7.0 | 13.6 | 468 | 506 |

(Go was not changed; its differences are run-to-run noise on a shared
machine.) Wall time improves 8 to 15% at the median and the tail shrinks; the
instruction count drops by 43% (about 45 M to 25.7 M, Callgrind), so the
wall-clock gain is far smaller than the instruction gain: a 6 ms run is
dominated by process start, dynamic loading, page faults and file I/O, which
are not instruction-bound. Step 2 alone: 26.77 M to 25.66 M instructions
(4%), not separable from noise in wall time.

### Callgrind after (sma-cross, 25.7 M Ir)

| Share of Ir | Where |
| --- | --- |
| 20% | `std::from_chars` (double) (was ~50% in `strtod`) |
| 6% | `parse_decimal_string` wrapper (inlined scanning) |
| 4% | Arrow `BinaryBuilder` append (Parquet string decode) |
| ~10% | `malloc`/`free` family |
| 3% | `memcpy` |
| 2% | Parquet `DeltaLengthByteArrayDecoder` |

### Tried and rejected

- Nothing was reverted. `-O2` vs `-O3`, LTO and `-march` were not tried:
  compute is a few percent of the run and `-march=native` is ignored in Nix.

### Remaining candidates

1. Read only the needed columns (`open`, `high`, `low` are unused by
   sma-cross); saves Parquet decode and parsing for them. Needs the strategy
   to declare its columns, a larger API change.
2. Avoid the Arrow `Table` and decode string columns directly (Parquet
   low-level reader) to cut the builder and malloc cost (~14%).
3. Remaining wall time is process start and fixed costs; measure with
   `perf stat` before more tuning. Static linking could cut dynamic loading.
4. Vectorised indicator columns: only for multi-instance sweeps.

## DATA.01: decimal candle columns

Issue #131. The lake stores prices as DECIMAL(18,8) and volume as DECIMAL(28,8)
instead of strings (format in `docs/ingest/binance-public-klines.md`). The C++
reader accepts both schemas by column type (`ValueColumn` in
`bars_parquet.cpp`); decimals below 2^53 unscaled convert with one exact
division, larger ones through the string path.

Same bench lake (seed 110, 5840 bars, 2 files), old string schema converted with
`task data:migrate-candles`:

| | strings (before) | decimals (after) |
|---|---|---|
| lake size, 2 files | 330,328 B | 154,188 B (-53%) |
| Go read (TI `lake.Read`, 2 files, median of 30) | 3.42 ms | 2.09 ms |
| C++ sma-cross median / p95 (100 runs) | 6.1 / 7.8 ms | 4.8 / 5.5 ms |
| C++ signal-consensus median / p95 (100 runs) | 6.5 / 7.4 ms | 5.2 / 5.9 ms |

Encodings tried on the same data (one file, bytes): plain + Snappy 100,363;
DELTA_BINARY_PACKED prices + Snappy 76,594 (chosen); additionally
DELTA_BYTE_ARRAY volume 75,011 (not worth it); plain uncompressed 172,120.
Wall times were taken on a shared machine (load 9 to 24); read them as a
direction, not a precise ratio.

`RunReport` stdout sha256 is identical for old binary on old lake, new binary on
new lake and new binary on old lake (`b39091c7...` sma-cross, `5a577dac...`
consensus). `genlakefixture` writes `fixtures/lake_legacy` in the old schema; the
`ParquetBars.DecimalAndLegacySchemasAgree` test checks both load to bit-identical
doubles, including a volume above int64 at scale 8.

## Gaps

- Go benchmark lives in `src/backtest/bench_test.go` and skips without
  `BENCH_LAKE_DIR`; it is not part of `go test` runs.
- One synthetic symbol, one config per strategy; trade counts and results of
  Go and C++ were not cross-checked in this phase.
- Single-process timings include a warm page cache; cold-cache runs not measured.

## 3000-instance experiment, end to end (issue #158, parent #130)

Target of #130: 3000 instances in about 10 s, down from about 5 min on the Go path.

Harness: `TestBenchPersistGrid` in `tests/system/experiment/bench_persist_test.go` (manual, skipped
unless `BENCH_INSTANCES` is set). It calls `backtest.RunExperiment` on a 3000-instance ma-cross grid
(10 SMA pairs x stop multiples, `gridConfig`) over one lake of 17520 hourly BTC/USDT bars (730 days).
`BENCH_WALK=1` selects a seeded random walk (seed 110); the default zigzag fixture is the same as the
system tests. Both fixtures trade a lot (about 1750 fills per instance on the walk, 1950 on the zigzag,
hourly SMA 3/6 to 24/48), so the persisted row volume is far above a realistic experiment.

```bash
task cpp:deps PRESET=optimized && task cpp:build PRESET=optimized
go test -c -o /tmp/exp.test ./tests/system/experiment
export BENCH_HOURS=17520 BENCH_WALK=1 CPP_SMA_CROSS_BIN=$PWD/build/optimized/strategies/sma-cross/strategy-sma-cross
BENCH_INSTANCES=3000 EXPERIMENT_ENGINE=cpp /tmp/exp.test -test.run BenchPersistGrid -test.v
# Postgres: add DATABASE_URL=... (a database loaded with db/sqlc/schema.sql; the harness mirrors the grid into it)
```

Timed: the `RunExperiment` call only, i.e. prepare (bar range, coverage), bar loading, the sweep
(C++ processes or Go loop), KPI replay, report.json, persistence (SQLite catalog or Postgres COPY),
progress updates. Not timed: fixture and catalog creation, process start of the test binary. S3 upload
is not exercised (no `EXPERIMENT_ARTIFACTS` bucket), and the Go `prepare` indicator pass is part of the Go number.
Machine: Intel Core Ultra 9 185H, 22 logical CPUs, 62 GB, Linux 7.0, Go 1.24, optimized C++ build, default
`EXPERIMENT_CPP_PROCESSES` (NumCPU) and `EXPERIMENT_CPP_SHARD_MAX` (500). The machine was shared: load
average 7 to 14 during all runs, so treat the numbers as upper bounds.

| run | store | instances | runs (s) | median (s) |
|---|---|---|---|---|
| Go, zigzag lake | SQLite | 3000 | 253 (one run) | 253 |
| Go, walk lake | SQLite | 100 | 15.1, 15.7, 10.9 | 15.1 (extrapolated to 3000: about 450) |
| cpp, walk lake | SQLite | 100 | 2.2, 2.3, 2.5 | 2.3 |
| cpp, walk lake | SQLite | 3000 | 12.1 (cold), 5.7, 5.6 | 5.7 |
| cpp, zigzag lake | SQLite | 3000 | 8.3 (cold), 8.5, 4.2, 4.3, 3.9, 4.0 | 4.2 |
| cpp, zigzag lake | Postgres | 3000 | 55, 66, 69 | 66 |
| cpp, walk lake | Postgres | 3000 | 229 (one run) | 229 |

The Go 3000 figure is measured on the zigzag lake only; the walk-lake Go figure is a linear extrapolation
from 100 instances (not measured). Go on Postgres at 100 instances: 15.2 and 7.9 s, same order as SQLite.

Result: with the SQLite catalog the C++ engine meets the target (about 4 to 6 s warm, one cold first run
8 to 12 s). With Postgres it does not: 66 s to 229 s. The time is in `PersistInstanceBatch` (the COPY of
5 M fills and 2.6 M trade outcomes), not in compute: per 200-instance batch the commit takes 0.1 to 0.17 s
on SQLite and 3 to 10 s on Postgres, report.json writing 3 ms. The CPU profile of the Postgres run shows
about 31 percent in pgx `CopyFrom` value encoding (`encodePlanDriverValuer`, `newEncodeError`: a
`uuid.UUID` per row goes through the `driver.Valuer` path) and about 21 percent in GC.

Equality, Go vs cpp (100 instances of the walk grid, Postgres-persisted metrics and fill counts, all 100
completed): Sharpe, Sortino, CAGR, max drawdown, monthly/yearly return, trades per month, wins, losses,
breakeven, win rate and fill count are identical (max relative difference 0). `metric_duration_ms` differs
by design (wall time of the instance).

Follow-ups (not done here): encode UUIDs as `pgtype.UUID`/`[16]byte` in the COPY rows and drop the
per-fill `uuid.New()`; consider COPY of fills only for sampled instances or a coarser row model; run the
Postgres case against a database without the dev stack on the same host. The fixture trades 10 to 20 times
more than real data, so the Postgres number is a worst case.
