# calculation (C++)

Build with a preset from `src/calculation`: `cmake --preset debug|optimized`, then `cmake --build ../../build/<preset>`.

## Valgrind and Callgrind

Opt-in targets, not part of the default build. Both run each registered strategy on the seeded fixture lake `tests/fixtures/lake_roundtrip` (decimal columns; `tests/fixtures/lake_legacy` keeps the old string schema for the dual-read test, both made by `testdata/genlakefixture`) (the Valgrind devbox package provides `valgrind`).

| Target | Preset | What it does |
| --- | --- | --- |
| `valgrind` | debug | memcheck (`--leak-check=full --track-origins=yes --error-exitcode=1`; only definite and indirect leaks fail, since protobuf and abseil report "possibly lost" interior pointers) on `dagobert-tests` without the system cases, then on each strategy binary |
| `callgrind` | optimized | callgrind on each strategy binary, then `callgrind_annotate`; writes `build/optimized/callgrind.<exe>.out` and `.out.txt` |

Invoking the target of the other preset fails with a message naming the right preset.

Add a strategy by calling `dagobert_register_strategy_run(<exe> <fixture-config.json>)` in its `CMakeLists.txt` (see `strategies/sma-cross`). Executables for the experiment engine are picked via `CPP_SMA_CROSS_BIN` (`build/<preset>/strategies/sma-cross/strategy-sma-cross`) and `CPP_SIGNAL_CONSENSUS_BIN` (`build/<preset>/strategies/consensus/strategy-signal-consensus`); the `dagobert-backtest` image sets both.

Benchmark and baseline: `docs/development/05-cpp-benchmark.md` (`task cpp:bench`).

## C++ dependency build

## Builds never compile dependencies

Arrow, Parquet, protobuf, abseil, thrift, snappy and zlib are built **once**
into a shared install prefix, `${DAGOBERT_DEPS_ROOT}/<key>` (default
`${XDG_CACHE_HOME:-~/.cache}/dagobert/deps`). Worktrees only hold source and
their own object files.

```bash
devbox shell
task cpp:deps PRESET=debug      # once per key (PRESET=optimized for release)
task cpp:build PRESET=debug     # configure + build
task cpp:test PRESET=debug      # build + ctest
```

- `<key>` = `<build type>-<hash>`; the hash covers compiler id+version, the
  build type, `deps/CMakeLists.txt` (versions, flags), the Arrow patch script
  and `cmake/DepsKey.cmake`. Bumping a version or flag creates a new prefix.
- `task cpp:deps` serialises on `flock $DAGOBERT_DEPS_ROOT/.lock`, builds into
  `<key>.partial` and renames it into place, so a half-built prefix is never
  visible. Concurrent callers wait, then find it done.
- Configuring `src/calculation` with a missing prefix fails with a message
  pointing at `task cpp:deps`; there is no per-worktree Arrow compile.
  The prefix is located from the key (not a fixed preset path) and prepended to
  `CMAKE_PREFIX_PATH` by `cmake/DepsKey.cmake`.
- Targets are unchanged for the code: `arrow_static`, `parquet_static`
  (from the package configs),
  `protobuf::libprotobuf`.
- googletest, magic_enum and StratForge are small and still use FetchContent.

## Portable build (container images)

The optimized preset tunes for the build host (`-march=native`). For an image built elsewhere set `DAGOBERT_PORTABLE=1` for `deps/build.sh` and `-DDAGOBERT_PORTABLE=ON` for the main configure; both use `-mtune=generic` instead. The key gets a `-portable` marker, so the prefix never collides with the host-tuned one. `DAGOBERT_CC` / `DAGOBERT_CXX` override the compiler of `deps/build.sh`. `docker/backtest/Dockerfile` does this in two stages: `cpp-deps` (recipe files only, cached) and `cpp-build`.

## Toolchain

Clang 23 + lld 23 + clang-tools 23 come from devbox, pinned to nixpkgs rev
`31ecbaca…` (prebuilt in cache.nixos.org), because devbox has no versioned
Clang 23 package. Clang 22 and 23 collide in a devbox profile, so only one is
installed. The linker is mold by default; `-DDAGOBERT_LINKER=lld` selects lld.
After switching toolchains, run `task cpp:deps` again once (about 10 min for
both presets, see the deps rows above) and delete existing build dirs.
Decision: adopted as default based on a one-off measurement (one run each,
Ninja, all cores): optimized cold 89 s vs 120 s, relink 73 s vs 89 s; debug on
par (cold 32.5 s vs 24.0 s, relink 3.8 s vs 3.9 s); no repeated benchmark and
no engine-runtime comparison yet. The backtest Docker image (Debian clang 19)
is unchanged.

## ccache

If `ccache` is installed (devbox provides it) the compiler launcher is
`cmake/ccache-launcher.sh`: shared cache `${XDG_CACHE_HOME:-~/.cache}/ccache`
(20G), `CCACHE_BASEDIR` = checkout root so worktrees share hits. Bypass with
`CCACHE_DISABLE=1`. The dependency build uses it too.

## Include hygiene (IWYU)

- `task cpp:iwyu` reports include-what-you-use findings for our `.cpp` files
  (report only, nothing fails).
- `task cpp:iwyu:check` fails on unnecessary includes that are not in
  `iwyu-baseline.txt`. It runs as the CI job `cpp-static-analysis`
  (`.github/workflows/cpp-static-analysis.yml`) and as the pre-commit hook
  `calculation-iwyu` on staged `.cpp` files (headers are covered by CI).
- `task cpp:iwyu:baseline` regenerates `iwyu-baseline.txt`.
- Policy (ratchet): only "should remove" findings are enforced. "Should add"
  suggestions are ignored. `iwyu-baseline.txt` lists the unnecessary includes
  that predate the check. It only ever shrinks; PR4a (BUILD.04) empties it.
  Never add entries to it.
- Fixing a finding: remove the include. If IWYU is wrong, keep it with
  `// IWYU pragma: keep`, or add a mapping in `iwyu.imp` (private header to
  public header; format described in the file).
- IWYU 0.26 is built on clang 22; the compiler is Clang 23. It works against
  our compile database; builtin headers differ only slightly.
- Usage: `PRESET=optimized scripts/cpp-iwyu.sh check src/calculation/sdk/x.cpp`
  checks one file (one-shot, same rules as the full check).

## One build per build directory

Do not run two builds in the same build directory (a concurrent build
truncated `libarrow.a` before). `task cpp:build` takes `flock` on
`build/<preset>.lock`.

## Measured times

The table below was measured with Clang 22.1.8 + lld (before the switch to
Clang 23 + mold, see Toolchain). Machine: 22 cores, Ninja, medians of 3, default
target (`all`), configure excluded (about 4 s per build dir). Before (issue #109):
about 25 minutes per fresh worktree build, Arrow compiled in every build dir.
Cold debug build with Make (no generator set, `-j1`) took 144 s.

The presets use Ninja. A build dir created earlier with Make must be deleted
(`rm -rf build/<preset>`), because CMake refuses to switch the generator of an
existing build dir.

| Step | Time |
| --- | --- |
| `task cpp:deps` debug, ccache warm from earlier attempts, once | 4m21s |
| `task cpp:deps` optimized, `CCACHE_DISABLE=1` (cold), once | 4m01s |
| `task cpp:deps` debug, Clang 23 + mold, once | 4m18s |
| `task cpp:deps` optimized, Clang 23 + mold, once | 5m39s |

| Preset | Cold (ccache off) | Cached (ccache warm, hits/total) | Relink (touch `sdk/report.cpp`, ccache hit) |
| --- | --- | --- | --- |
| debug | 26.6s | 1.0s (59/59) | 0.7s |
| optimized | 1m50s | 1m28s (59/59) | 1m25s |

Relink rebuilds every executable, since `sdk/report.cpp` is in
`dagobert_strategy`. With ccache off it also recompiles the file: 3.8s for
debug. Optimized cached and relink times are dominated by ThinLTO at link time.
