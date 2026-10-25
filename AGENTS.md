# AGENTS.md

## Project documentation
- Development workflow lives in `docs/development/`.
- Keep documentation consistent with the implementation.

## Before planning or implementing
1. For tracked work, follow `docs/development/01-task-management.md`.
2. GitHub operations go through the GitHub MCP server: `docs/development/02-github-mcp.md`.
3. Follow `docs/development/principles/README.md` for planning and implementation: applicability, simplicity, twelve-factor, clean code, scalability, and the Definition of done. Check the Definition of done before closing a non-trivial task.
4. Follow `docs/development/04-coding-principles.md` for job containers. System tests on the job container prove the result. An audit covers edge cases, performance, and clean code.

Trivial changes (typos, docs wording) can skip the full workflow.

## C++ builds
- Builds never compile dependencies. Run `task cpp:deps PRESET=<debug|optimized>` once; configure then fails with a pointer to it if the shared prefix is missing. Details: `src/calculation/README.md`.

## Project board
- The GitHub Project for this repository (`Dagobert`, number 5, see `docs/development/01-task-management.md`) must reflect the current state of agentic development at all times.
- Set an issue to `In progress` directly when work on it starts, before planning or coding, and update its status on every later change (`In review` once its pull request is open, back to `In progress` when it is reworked).

## Pull requests
- When opening a pull request for an issue, prefix the title with the issue ID from the issue title, for example `[00.2] <task name>`.
- Reference the issue with a closing keyword such as `Closes #42`.

## Pull request size
Every pull request must be reviewable by a human in one sitting.
- Aim for at most ~600 changed lines, not counting lockfiles and generated files. Treat ~800 as a hard limit.
- One concern per pull request. Do not mix refactoring, formatting or dependency bumps with behaviour changes.
- Check the expected size while planning. If an issue will exceed the limit, propose splitting it into smaller issues or stacked pull requests before implementing, each one independently green and reviewable.
- Do not add scope found during implementation; propose a follow-up issue instead.
- In the description, state what changed and why, and point to where to start reviewing.

## Worktrees
Issue, pull request and worktree relate 1:1:1. An issue split into stacked pull requests gets one worktree per pull request.
- Work on one pull request only in its own git worktree, on its own branch. Never let two pull requests, issues or agents share a worktree, and never edit a worktree you did not create.
- Worktrees live in the gitignored `worktree/` folder at the repo root. Name each worktree after its branch, for example `worktree/feat-89-go-sizing-core` for `feat/89-go-sizing-core`.
- After the pull request is merged, remove its worktree (`git worktree remove`) and its local branch (`git branch -d`).
- Before creating a worktree or starting an agent, check that no other session or agent already owns the issue. If one does, ask it instead of starting in parallel.
- Remove the worktree (`git worktree remove`) once its branch is pushed and work stops: pull request in review, merged, closed, superseded or paused. The pushed branch is the durable state; to rework, create a fresh worktree from it. Never remove a worktree with uncommitted changes without saving them first.

## Subagents and models
Agentic work is split by model to save tokens (`docs/development/03-agent-harness.md`):
- Grooming, planning and validation run on a stronger model.
- All execution runs on a cheaper model: implementation, exploration and chores such as rebases, checks, board and PR bookkeeping.
- The main session orchestrates and delegates. Keep it from reading or editing code itself beyond what a decision needs.
- Hand the worker concrete work units from a plan, never open design questions. A validation failure goes back to the worker as its next work unit.
- Skip the harness for trivial changes.

## Before finishing
- Make the smallest correct change, run the relevant tests and checks, and review the diff.
- Update documentation when behaviour, architecture or workflows change.
