#!/usr/bin/env bash
# PreToolUse guard: block reads and writes of credential files (.secrets, .env).
# Claude Code passes the tool call as JSON on stdin. Exit 2 blocks the call and
# sends the message on stderr back to Claude.
set -euo pipefail

input="$(cat)"
text="$(printf '%s' "$input" | jq -r '[.tool_input.file_path? // empty, .tool_input.command? // empty] | join(" ")')"

# Matches ".secrets", ".env", ".env.local" as a whole path segment, not ".envrc" or "env.example".
if printf '%s' "$text" | grep -Eq '(^|[/[:space:]])\.(secrets|env)([.[:space:]/]|$)'; then
  echo "Blocked: .secrets and .env hold credentials. Use env.example to see the variable names." >&2
  exit 2
fi
exit 0
