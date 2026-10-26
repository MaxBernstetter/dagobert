# C++ vs Go benchmark (CALC.25, phase 1: baseline)

Baseline measurements for issue #110. Nothing here optimises code; the
candidates at the end are evidence-based proposals for phase 2.

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

## Optimisation candidates (not implemented)

1. Faster decimal parsing in `load_bars_from_lake`: replace `std::stod` with
   `std::from_chars` (or Arrow's fast float parse). Targets ~50% of
   instructions. Measure: expected gain up to ~2x on the whole run.
2. Avoid per-value `std::string` copies when reading the Arrow string
   columns (read as `string_view`/buffer slices, reserve the bar vector).
3. At 6 to 7 ms the run is also bound by process start and Parquet footer
   work; check `perf stat`/`strace -c` before touching compiler flags.
4. Compiler flags (`-O2` vs `-O3`, LTO, `-march`): compute is ~3% of the run,
   so expect little; `-march=native` is not applied in the Nix shell anyway.
   Verify with the harness before keeping any flag.
5. Vectorised indicator columns: not justified by this profile at this size;
   revisit with multi-instance sweeps where indicator work repeats per config.

Go side, for the experiment path: `prepare` (~200 ms) recomputes all indicators
per bar over a rolling window and JSON-roundtrips them; this is where the
production gap comes from, not language speed alone.

## Gaps

- Go benchmark lives in `src/backtest/bench_test.go` and skips without
  `BENCH_LAKE_DIR`; it is not part of `go test` runs.
- One synthetic symbol, one config per strategy; trade counts and results of
  Go and C++ were not cross-checked in this phase.
- Single-process timings include a warm page cache; cold-cache runs not measured.
