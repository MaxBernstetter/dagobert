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
prices stored as strings like the real lake), runs each optimized strategy
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

## Gaps

- Go benchmark lives in `src/backtest/bench_test.go` and skips without
  `BENCH_LAKE_DIR`; it is not part of `go test` runs.
- One synthetic symbol, one config per strategy; trade counts and results of
  Go and C++ were not cross-checked in this phase.
- Single-process timings include a warm page cache; cold-cache runs not measured.
