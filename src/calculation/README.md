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
