# dagobert-dev Claude Code plugin

Agents and the `/ready` command for this repository. The hooks are not here: they are configured once in `.claude/settings.json` at the repository root.

## What it contains

| Path | What it does |
|---|---|
| `hooks/guard-secrets.sh` | PreToolUse script. Blocks Read, Edit, Write and Bash calls that touch `.secrets` or `.env`. Wired in `.claude/settings.json`. |
| `hooks/go-format.sh` | PostToolUse script. Runs `gofmt -w` on an edited Go file and `go vet` on its package. Wired in `.claude/settings.json`. |
| `commands/ready.md` | `/dagobert-dev:ready`. Runs the Definition of done checks for the current change. |
| `agents/groomer.md` | Gets an issue to Ready, on Opus 5.5. Read-only; reads GitHub through the MCP server. |
| `agents/planner.md` | Turns a ready issue into pull-request-sized work units, on Opus 5.5. Read-only. |
| `agents/worker.md` | Execution agent on Haiku 5.5, per `docs/development/03-agent-harness.md`. Cannot spawn agents. |
| `agents/validator.md` | Read-only review agent on Sonnet 5.5, per the same harness doc. |

## Load it

The agents and `/ready` need the plugin loaded:

```bash
claude --plugin-dir /home/max.bernstetter/personal/dagobert/tools/claude-plugin
```

The hook scripts need `jq` and `gofmt`/`go` on `PATH`. `devbox shell` provides Go; `jq` is in `/usr/bin`.

## Test the guard

```bash
echo '{"tool_input":{"file_path":"/x/.secrets"}}' | tools/claude-plugin/hooks/guard-secrets.sh; echo "exit $?"   # expect exit 2
echo '{"tool_input":{"file_path":"/x/env.example"}}' | tools/claude-plugin/hooks/guard-secrets.sh; echo "exit $?"  # expect exit 0
```

## Notes

- Project agents and commands cannot live in `.claude/` in this checkout: the sandbox replaces those subfolders with read-only placeholders. This plugin is the place for them.
- The agents pin model IDs from the environment list. Update them there if the model family changes.
