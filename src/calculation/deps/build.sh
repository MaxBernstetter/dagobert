#!/usr/bin/env bash
# Build the shared dependency prefix for a preset (debug|optimized).
# Serialised with flock; installs atomically (build into <key>.partial, then
# rename). Usage: deps/build.sh [debug|optimized]
set -euo pipefail

preset="${1:-debug}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
case "$preset" in
  debug) type=Debug ;;
  optimized) type=Release ;;
  *) echo "unknown preset '$preset' (debug|optimized)" >&2; exit 2 ;;
esac

root="${DAGOBERT_DEPS_ROOT:-${XDG_CACHE_HOME:-$HOME/.cache}/dagobert/deps}"
mkdir -p "$root"
exec 9>"$root/.lock"
flock 9 # concurrent callers wait here, then find the prefix already built

args=(-G Ninja -S "$here" -DDAGOBERT_MODE="$preset" -DCMAKE_BUILD_TYPE="$type"
  -DCMAKE_C_COMPILER=clang -DCMAKE_CXX_COMPILER=clang++
  -DDAGOBERT_DEPS_ROOT="$root")

probe="$root/.work/probe-$preset"
cmake -B "$probe" "${args[@]}" >/dev/null
key="$(cat "$probe/key.txt")"
if [ -f "$root/$key/.complete" ]; then
  echo "deps up to date: $root/$key"
  exit 0
fi

work="$root/.work/$key"
rm -rf "$work" "$root/$key.partial" "$root/$key"
cmake -B "$work" "${args[@]}" -DDAGOBERT_DEPS_STAGING=ON
cmake --build "$work"
touch "$root/$key.partial/.complete"
mv "$root/$key.partial" "$root/$key"
rm -rf "$work"
echo "deps installed: $root/$key"
