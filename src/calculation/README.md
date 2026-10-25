# calculation (C++)

Build with a preset from `src/calculation`: `cmake --preset debug|optimized`, then `cmake --build ../../build/<preset>`.

## Valgrind and Callgrind

Opt-in targets, not part of the default build. Both run each registered strategy on the seeded fixture lake `tests/fixtures/lake_roundtrip` (the Valgrind devbox package provides `valgrind`).

| Target | Preset | What it does |
| --- | --- | --- |
| `valgrind` | debug | memcheck (`--leak-check=full --track-origins=yes --error-exitcode=1`; only definite and indirect leaks fail, since protobuf and abseil report "possibly lost" interior pointers) on `dagobert-tests` without the system cases, then on each strategy binary |
| `callgrind` | optimized | callgrind on each strategy binary, then `callgrind_annotate`; writes `build/optimized/callgrind.<exe>.out` and `.out.txt` |

Invoking the target of the other preset fails with a message naming the right preset.

Add a strategy by calling `dagobert_register_strategy_run(<exe> <fixture-config.json>)` in its `CMakeLists.txt` (see `strategies/sma-cross`).

# C++ calculation build

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

## ccache

If `ccache` is installed (devbox provides it) the compiler launcher is
`cmake/ccache-launcher.sh`: shared cache `${XDG_CACHE_HOME:-~/.cache}/ccache`
(20G), `CCACHE_BASEDIR` = checkout root so worktrees share hits. Bypass with
`CCACHE_DISABLE=1`. The dependency build uses it too.

## One build per build directory

Do not run two builds in the same build directory (a concurrent build
truncated `libarrow.a` before). `task cpp:build` takes `flock` on
`build/<preset>.lock`.

## Measured times

Machine: 22 cores, Ninja/Make defaults, clang 22.1.8. Before (issue #109):
about 25 minutes per fresh worktree build, Arrow compiled in every build dir.

| Step | Time |
| --- | --- |
| `task cpp:deps` debug, ccache warm from earlier attempts, once | 4m21s |
| `task cpp:deps` optimized, `CCACHE_DISABLE=1` (cold), once | 4m01s |
| fresh worktree: configure + build `dagobert-tests` and `strategy-sma-cross` + ctest (51 tests), debug, ccache off | 1m44s |
| same, ccache cold-ish (28 of 46 compiles hit) | 1m07s |
| same, ccache fully warm (46 of 46 hit) | 7s (build only) |
| fresh worktree, optimized, ccache cold | 1m41s |
| rebuild after a CMake-only change (`touch CMakeLists.txt`) | 1s |

Timings were taken on a machine that was running other builds, so treat them
as upper bounds.
