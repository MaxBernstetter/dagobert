#!/usr/bin/env bash
# include-what-you-use over our src/calculation translation units. Only
# "should remove" findings are enforced, as a ratchet against
# src/calculation/iwyu-baseline.txt; "should add" suggestions are ignored.
# Usage: [PRESET=<preset>] scripts/cpp-iwyu.sh [report|check|baseline] [files...]
# Needs: build/<preset>/compile_commands.json (cmake --preset <preset> -S src/calculation),
# iwyu_tool.py and iwyu (devbox), python3.
set -euo pipefail
cd "$(dirname "$0")/.."
ROOT="$PWD"
MODE="${1:-report}"
[ $# -gt 0 ] && shift
PRESET="${PRESET:-debug}"
DB="build/$PRESET/compile_commands.json"
BASELINE="src/calculation/iwyu-baseline.txt"
MAPPING="$ROOT/src/calculation/iwyu.imp"

case "$MODE" in
  report | check | baseline) ;;
  *) echo "usage: $0 [report|check|baseline] [files...]" >&2; exit 2 ;;
esac
if [ "$MODE" = baseline ] && [ $# -gt 0 ]; then
  echo "baseline takes no file arguments (it always covers all TUs)" >&2
  exit 2
fi
if [ ! -f "$DB" ]; then
  echo "$DB is missing; configure with: cmake --preset $PRESET -S src/calculation" >&2
  exit 1
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# 1. Our TUs from the compile database (optionally restricted to the given files).
python3 -I - "$DB" "$ROOT" "$TMP/tus.txt" "$@" <<'PY'
import json, os, sys
db, root, out, args = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4:]
def rel(p):
    return os.path.relpath(os.path.realpath(p), root)
ours = set()
for e in json.load(open(db, encoding="utf-8")):
    f = e["file"] if os.path.isabs(e["file"]) else os.path.join(e.get("directory", ""), e["file"])
    r = rel(f)
    if r.startswith("src/calculation/") and not any(s in "/" + r + "/" for s in ("/gen/", "/third_party/", "/_deps/")):
        ours.add(r)
if args:
    ours &= {rel(a) for a in args}
with open(out, "w", encoding="utf-8") as fh:
    fh.write("".join(f"{r}\n" for r in sorted(ours)))
PY
mapfile -t TUS < "$TMP/tus.txt"
if [ "${#TUS[@]}" -eq 0 ]; then
  if [ $# -gt 0 ]; then exit 0; fi # none of the given files is a TU of ours
  echo "no src/calculation translation units in $DB" >&2
  exit 1
fi

# 2. IWYU does not support precompiled headers. Give it a copy of the compile
# database without the PCH flags (-include-pch / -include, -Winvalid-pch); the
# build's own compile_commands.json is left as it is.
mkdir -p "$TMP/db"
python3 -I - "$DB" "$TMP/db/compile_commands.json" <<'PY'
import json, shlex, sys
db, out = sys.argv[1], sys.argv[2]
def strip_pch(args):
    keep, i = [], 0
    while i < len(args):
        a = args[i]
        if a == "-Winvalid-pch":
            i += 1
        elif a == "-Xclang" and i + 3 < len(args) and args[i + 1] in ("-include-pch", "-include") \
                and args[i + 2] == "-Xclang":
            i += 4
        else:
            keep.append(a)
            i += 1
    return keep
entries = json.load(open(db, encoding="utf-8"))
for e in entries:
    if "arguments" in e:
        e["arguments"] = strip_pch(e["arguments"])
    else:
        e["command"] = shlex.join(strip_pch(shlex.split(e["command"])))
json.dump(entries, open(out, "w", encoding="utf-8"))
PY

# 3. Run IWYU (never fails on findings; the parser decides).
RAW="$TMP/iwyu.txt"
rc=0
iwyu_tool.py -p "$TMP/db" -j "$(nproc)" "${TUS[@]}" -- \
  -Xiwyu --error=0 -Xiwyu "--mapping_file=$MAPPING" >"$RAW" || rc=$?
if [ "$rc" -ne 0 ]; then
  echo "iwyu_tool.py failed with exit $rc" >&2
  exit "$rc"
fi

# 4. Normalize "should remove" findings; check them against the baseline.
cat >"$TMP/parse.py" <<'PY'
import os, re, sys
mode, root, raw, target, scope_path = sys.argv[1:6]
HDR = re.compile(r"^(.+) should remove these lines:$")
INC = re.compile(r'^- (#include\s+(?:<[^>]+>|"[^"]+"))')
found = set()
cur = None
with open(raw, encoding="utf-8", errors="replace") as fh:
    for line in fh:
        line = line.rstrip("\n")
        m = HDR.match(line)
        if m:
            cur = m.group(1)
            continue
        m = INC.match(line)
        if m and cur is not None:
            rel = os.path.relpath(cur, root)
            if rel.startswith("src/calculation/") and not any(s in "/" + rel + "/" for s in ("/gen/", "/third_party/")):
                found.add(f"{rel}: {m.group(1)}")
            continue
        cur = None

def path_of(e):
    return e.split(": ", 1)[0]

if mode == "entries":
    with open(target, "w", encoding="utf-8") as fh:
        fh.write("".join(e + "\n" for e in sorted(found)))
elif mode == "baseline":
    with open(target, "w", encoding="utf-8") as fh:
        fh.write("# Unnecessary includes; kept empty. Do not add entries: remove the include or keep it with // IWYU pragma: keep. "
                 "Regenerate: task cpp:iwyu:baseline\n")
        fh.write("".join(e + "\n" for e in sorted(found)))
    print(f"wrote {len(found)} entries to {target}")
elif mode == "check":
    base = set()
    with open(target, encoding="utf-8") as fh:
        for line in fh:
            s = line.strip()
            if s and not s.startswith("#"):
                base.add(s)
    scope = None
    if scope_path != "-":
        with open(scope_path, encoding="utf-8") as fh:
            scope = {l.strip() for l in fh if l.strip()}
        found = {e for e in found if path_of(e) in scope}
        base = {e for e in base if path_of(e) in scope}
    new = sorted(found - base)
    for e in new:
        print(f"{e} is not needed (include-what-you-use)")
    if new:
        print("Remove the include. If IWYU is wrong, keep it with an IWYU pragma (// IWYU pragma: keep) "
              "or add a mapping in src/calculation/iwyu.imp. Do not add to the baseline.")
        sys.exit(1)
    if scope is None:
        for e in sorted(base - found):
            print(f"baseline entry fixed, remove it: {e}")
PY

case "$MODE" in
  report)
    cat "$RAW"
    python3 -I "$TMP/parse.py" entries "$ROOT" "$RAW" "$TMP/entries.txt" -
    echo "$(grep -c . "$TMP/entries.txt" || true) unnecessary include(s)"
    ;;
  baseline)
    python3 -I "$TMP/parse.py" baseline "$ROOT" "$RAW" "$ROOT/$BASELINE" -
    ;;
  check)
    scope="-"
    [ $# -gt 0 ] && scope="$TMP/tus.txt"
    python3 -I "$TMP/parse.py" check "$ROOT" "$RAW" "$ROOT/$BASELINE" "$scope"
    ;;
esac
