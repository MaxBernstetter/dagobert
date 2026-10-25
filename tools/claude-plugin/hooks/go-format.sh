#!/usr/bin/env bash
# PostToolUse: format an edited Go file and vet its package. Formatting is
# applied in place. Vet findings are reported back to Claude on stderr.
set -uo pipefail

file="$(jq -r '.tool_input.file_path // empty')"
[[ "$file" == *.go ]] || exit 0
[[ -f "$file" ]] || exit 0

gofmt -w "$file" 2>&1 || true

pkg_dir="$(dirname "$file")"
vet_rc=0
vet_out="$(cd "$pkg_dir" && go vet ./... 2>&1)" || vet_rc=$?
# A read-only Go build cache (sandboxed runs) prints a cache-trim warning that is not a vet finding.
vet_out="$(printf '%s\n' "$vet_out" | grep -v '^go: failed to trim cache' || true)"
if [[ $vet_rc -ne 0 && -n "$vet_out" ]]; then
  printf 'go vet found issues in %s:\n%s\n' "$pkg_dir" "$vet_out" >&2
  exit 2
fi
exit 0
