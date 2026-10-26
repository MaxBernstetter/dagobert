#!/usr/bin/env bash
# Benchmark the optimized C++ strategy executables against the Go experiment
# path on one seeded lake. Usage: scripts/cpp-bench.sh [runs]  (default 15)
# Needs: task cpp:build PRESET=optimized, /usr/bin/time, python3.
# Output: build/bench/results.json (raw samples) and build/bench/results.md.
set -euo pipefail
cd "$(dirname "$0")/.."
RUNS="${1:-15}"
OUT=build/bench
LAKE="$PWD/$OUT/lake"
FROM=1704067200000 # 2024-01-01 UTC
TO=1767139200000   # 2025-12-30 UTC (730 days)

rm -rf "$LAKE"
go run ./src/calculation/testdata/genlakefixture -bench "$LAKE" -bars 5840

cfg() { # label payload-json
  cat >"$OUT/$1.json" <<JSON
{"schemaVersion":2,"label":"$1","provider":"binance","base":"BTC","quote":"USDT","fromMs":"$FROM","toMs":"$TO","initialQuote":1000.0,"payload":$2}
JSON
}
cfg sma-cross '{"@type":"type.googleapis.com/tradingcopilot.v1.MaCrossConfig","fastHr":12,"slowHr":48}'
cfg signal-consensus '{"@type":"type.googleapis.com/tradingcopilot.v1.SignalConsensusConfig"}'

go test -c -o "$OUT/gobench.test" ./src/backtest
: >"$OUT/go.txt"
for s in sma-cross signal-consensus; do
  for st in prepare simulate end_to_end; do
    BENCH_LAKE_DIR="$LAKE" "$OUT/gobench.test" -test.run xxx \
      -test.bench "GoExperiment/$s/$st\$" -test.benchtime="${RUNS}x" -test.benchmem |
      grep '^BenchmarkGoExperiment' >>"$OUT/go.txt"
  done
done

python3 -I - "$OUT" "$RUNS" "$LAKE" <<'PY'
import json, os, subprocess, sys, time, statistics as st
out, runs, lake = sys.argv[1], int(sys.argv[2]), sys.argv[3]
def run_cpp(s):
    exe = subprocess.check_output(["find", "build/optimized", "-name", "strategy-" + s, "-type", "f"], text=True).split()[0]
    for _ in range(runs):
        t0 = time.perf_counter()
        p = subprocess.Popen([exe, "--config", f"{out}/{s}.json", "--data-dir", lake], stdout=subprocess.DEVNULL)
        _, status, ru = os.wait4(p.pid, 0)
        assert status == 0, status
        yield (time.perf_counter() - t0) * 1000, ru.ru_maxrss

def p95(v):
    v = sorted(v); return v[min(len(v) - 1, -(-len(v) * 95 // 100) - 1)]
res = {"cpp": {}, "go": {}}
raw = {}
for s in ("sma-cross", "signal-consensus"):
    raw[s] = list(run_cpp(s))
for s, v in raw.items():
    t = [x[0] for x in v]
    res["cpp"][s] = {"runs": len(v), "median_ms": st.median(t), "p95_ms": p95(t), "peak_rss_kb": max(x[1] for x in v)}
for line in open(out + "/go.txt"):
    f = line.split(); name = f[0].split("/")
    m = {f[i + 1]: float(f[i]) for i in range(2, len(f) - 1, 2)}
    res["go"].setdefault(name[1], {})[name[2].rsplit("-", 1)[0]] = {
        "median_ms": m["median_ms"], "p95_ms": m["p95_ms"], "peak_rss_kb": m["peak_rss_kb"],
        "alloc_mb_per_op": m["B/op"] / 1e6, "allocs_per_op": m["allocs/op"]}
json.dump(res, open(out + "/results.json", "w"), indent=1)
md = ["| strategy | C++ median ms | C++ p95 ms | C++ RSS MB | Go e2e median ms | Go e2e p95 ms | Go e2e RSS MB | Go prepare ms | Go simulate ms | speedup (median) |", "|---|---|---|---|---|---|---|---|---|---|"]
for s, c in res["cpp"].items():
    g = res["go"][s]
    md.append(f"| {s} | {c['median_ms']:.1f} | {c['p95_ms']:.1f} | {c['peak_rss_kb']/1024:.1f} | {g['end_to_end']['median_ms']:.0f} | {g['end_to_end']['p95_ms']:.0f} | {g['end_to_end']['peak_rss_kb']/1024:.1f} | {g['prepare']['median_ms']:.0f} | {g['simulate']['median_ms']:.0f} | {g['end_to_end']['median_ms']/c['median_ms']:.0f}x |")
open(out + "/results.md", "w").write("\n".join(md) + "\n")
print("\n".join(md))
PY
