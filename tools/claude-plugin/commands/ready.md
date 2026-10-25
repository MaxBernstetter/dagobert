---
description: Run the Definition of done checks for the current change
allowed-tools: Bash(git status:*), Bash(git diff:*), Bash(gofmt:*), Bash(go vet:*), Bash(go test:*), Bash(task test:*), Bash(npm run build:*)
---

Check the current change against the Definition of done in `docs/development/principles/README.md`.

1. Run `git status --short` and `git diff --stat` to see what changed. If nothing changed, say so and stop.
2. Run `gofmt -l` on the changed Go files. Any output is a failure.
3. Run `go vet` on each changed Go package directory.
4. Run `go test` on each changed pure-logic package (`src/internal`, `src/shared`, `src/calculation-signal/playbooks`, `src/calculation-technical-indicator/internal/indicators`).
5. For each changed job under `src/<job>/`, run `task test` only if the job has a system test under `tests/system/`. Report the exit code and the failing test names if any.
6. If anything under `src/frontend/` changed, run `npm run build` in `src/frontend`.
7. Print the Definition of done checklist from `docs/development/principles/README.md`. Mark each item as met, not met, or not checked, with one reason. Do not mark an item met without evidence from steps 1 to 6.

Report results faithfully. If a check did not run, say so.
