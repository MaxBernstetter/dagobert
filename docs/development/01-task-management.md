# Task Management Workflow

## Overview

GitHub Issues and GitHub Projects are the source of truth for project task tracking.

- **GitHub Issues** define bugs, features, improvements, and technical debt.
- **GitHub Projects** provide a consolidated view of tasks, priorities, and workflow status.
- **Pull requests** connect implementation changes to their corresponding issues.
- **Documentation** under `docs/` captures durable technical knowledge, architecture, and operational procedures.

Tasks should be traceable from initial requirements through implementation, verification, and completion.

## GitHub Project

All epics and tasks of this repository are tracked on one user-owned GitHub Project for `MaxBernstetter/dagobert`:

- Owner: `MaxBernstetter`
- Project number: `5` (Dagobert)
- URL: https://github.com/users/MaxBernstetter/projects/5

### Fields

| Field | Values | Use |
|---|---|---|
| Status | Backlog, Ready, In progress, In review, Done | Workflow state, see [2. Prioritize and prepare work](#2-prioritize-and-prepare-work) |
| Priority | P0, P1, P2 | P0 = blocking or most urgent, P2 = can wait |
| Size | XS, S, M, L, XL | Relative effort of a task |
| Estimate | number | Optional effort estimate |
| Start date / Target date | date | Planning for the Roadmap view |

Built-in fields such as Assignees, Labels, Linked pull requests, Parent issue, and Sub-issues progress are populated from the issues themselves.

### Views

| View | Layout | Purpose |
|---|---|---|
| Backlog | Board | Items grouped by Status |
| Priority board | Board | Items grouped by Priority |
| Team items | Table | All items with planning fields |
| Roadmap | Roadmap | Timeline by Start date / Target date |
| My items | Table | Items assigned to the current user (`assignee:@me`) |

## Workflow

### 1. Create an issue

Create a GitHub issue for each meaningful bug, feature, or improvement.

Each issue should describe:

- **Problem:** What needs to change and why?
- **Expected outcome:** What should be true when the work is complete?
- **Acceptance criteria:** How will completion be verified?
- **Documentation impact:** Which notes under `docs/` need to be created or updated?

Keep issues focused and small enough to implement and review independently.

Use labels to classify work, for example:

- `bug`
- `enhancement`
- `documentation`
- `technical-debt`

### Hierarchy and labels

Work is organized as **epics** and **tasks**:

- **Epic:** one issue per domain, labelled `epic`.
- **Task:** a native sub-issue of its epic, labelled with its domain.
- **Type:** each task has a type label (`enhancement`, `documentation`, ...).
- **Dependencies:** each task lists the issues that block it in a `## Blocked by` section of its body.

Title convention: `[EPIC-00] <domain name>` for epics, `[00.2] <task name>` for tasks.

GitHub operations by agents go through the GitHub MCP server, see [02-github-mcp](02-github-mcp.md).

### 2. Prioritize and prepare work

Add issues to the [GitHub Project](#github-project) and set **Status**, **Priority**, and **Size**.

| Status | Meaning |
|---|---|
| Backlog | Captured but not yet prioritized or ready |
| Ready | Prioritized, sufficiently specified, and ready to implement |
| In progress | Implementation is underway |
| In review | A pull request is awaiting review or verification |
| Done | Changes are merged and required checks have passed |

Before starting substantial work, ensure the issue has clear acceptance criteria and known dependencies.

### 3. Implement the issue

Before coding:

1. Read the relevant issue and its acceptance criteria.
2. Inspect the existing implementation, tests, and relevant documentation.
3. Check `AGENTS.md` and any applicable project skills.
4. Check applicability and the twelve-factor rules in [principles](principles/README.md) (problem, owner, need, fit).
5. Identify the smallest implementation that satisfies the requirements.
6. Assign the current user to the task and move its project item to `In progress`.

During implementation:

- Keep changes scoped to the issue.
- Add or update tests where appropriate.
- Update relevant notes under `docs/` when behavior, architecture, configuration, or workflows change.
- Record significant architectural decisions in the appropriate Architecture Decision Record (ADR).

If the requirements change materially, update the issue before expanding the implementation.

### 4. Open a pull request

Create a pull request that references the issue, for example:

`Closes #42`

Prefix the pull request title with the issue ID from the issue title, for example `[00.2] <task name>`.

The pull request should include:

- A concise summary of the change.
- The issue being addressed.
- Tests and verification performed.
- Documentation changes, if applicable.
- Known limitations or follow-up work.

Move the task to `In review` while review and required checks are pending.

### 5. Verify and close

Before considering the task complete, address review feedback, confirm the acceptance criteria, and check the [Definition of done](principles/README.md#definition-of-done).

Merge the pull request when the required checks and reviews pass. GitHub can automatically close the linked issue when a pull request containing a supported closing keyword is merged into the repository's default branch.

Move the project item to `Done` once the completion criteria are satisfied.

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
- Name the worktree after its branch, for example `feat-19-1-grid-reference` for `feat/19-1-grid-reference`.
- Before creating a worktree or starting an agent, check that no other session or agent already owns the issue (the board, existing worktrees). If one does, ask it instead of starting in parallel.
- Remove the worktree (`git worktree remove`) once its branch is pushed and work stops: pull request in review, merged, closed, superseded or paused. The pushed branch is the durable state; to rework, create a fresh worktree from it. Never remove a worktree with uncommitted changes without saving them first.

## AI Agent Responsibilities

Coding agents must:

- Use GitHub Issues as the source of truth for tracked work.
- Read the relevant issue before implementing substantial changes.
- Ask for clarification or identify unresolved requirements rather than inventing acceptance criteria.
- Avoid silently expanding the scope of an issue.
- Update tests and documentation when appropriate.
- Report verification results accurately.
- Never claim to have created, updated, assigned, or closed an issue unless the action actually succeeded.
- Keep the project board current: set an issue to `In progress` when work on it starts, before planning or coding, and update its status on every later change (`In review` once its pull request is open, back to `In progress` when it is reworked).

Agents may propose new issues for discovered bugs, technical debt, or follow-up improvements. Keep unrelated work separate from the current task.

## Documentation Boundaries

Use each system for its intended purpose:

- **GitHub Issues:** Requirements, acceptance criteria, task discussions, and follow-up work.
- **GitHub Projects:** Priorities, ownership, status, and planning.
- **Pull requests:** Implementation review, verification, and change history.
- **`docs/`:** Durable project knowledge, architecture, decisions, development instructions, and operational guidance.

Do not duplicate the entire issue tracker in Markdown files. Link to GitHub issues when referencing tracked work from documentation.

## Guiding Principle

Every substantial change should have a clear purpose, verifiable completion criteria, appropriate tests, and up-to-date documentation. Keep the workflow lightweight for trivial changes and apply the full process to work that warrants it.
