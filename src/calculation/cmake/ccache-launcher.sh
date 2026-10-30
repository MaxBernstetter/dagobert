#!/usr/bin/env bash
# Compiler launcher: ccache with a cache shared across worktrees. BASEDIR is
# the checkout root, so the same sources in different worktrees hit the cache.
# A single-program launcher because Arrow's bundled deps forward it unquoted.
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export CCACHE_BASEDIR="${CCACHE_BASEDIR:-$(cd "$here/../../.." && pwd)}"
export CCACHE_DIR="${CCACHE_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/ccache}"
export CCACHE_MAXSIZE="${CCACHE_MAXSIZE:-20G}"
export CCACHE_COMPILERCHECK="${CCACHE_COMPILERCHECK:-content}"
# Precompiled headers embed macros and timestamps; without these the PCH never hits.
export CCACHE_SLOPPINESS="${CCACHE_SLOPPINESS:-pch_defines,time_macros}"
exec ccache "$@"
