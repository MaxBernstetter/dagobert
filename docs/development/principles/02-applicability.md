# Applicability

Before designing anything, check that the piece **applies**: that it solves the real problem, belongs where it is planned, and is needed now.

## Questions before starting

1. **Problem:** which user or system problem does this solve? Is it written in the issue?
2. **Owner:** which job or package owns this behaviour (see the layout in [CLAUDE.md](../../../CLAUDE.md))? If the answer is "several", the cut is wrong.
3. **Need:** is it needed for the current phase, or only "nice to have later"? Later means a new issue, not code now.
4. **Fit:** does an existing job, package or playbook already cover it? Prefer extending what exists.
5. **Evidence:** how will we know it works? This becomes the acceptance criteria (see [05-testing](05-testing.md)).

## Choosing technology

- Pick a tool because the problem needs it, not because it's available. Every new dependency must be justified in the issue or an ADR.
- Prefer what the repository already uses: Go standard library and the existing modules, SQLite and Parquet on `./data`, Compose and Taskfile.
- A significant or hard-to-reverse choice gets an ADR under `docs/architecture/`.

## Cutting the work

- Cut **vertical slices**: a narrow path through every needed layer (ingest or calculation → store → runner → test) that is demoable on its own.
- A slice fits into one issue and one pull request.
- If requirements change during implementation, update the issue first, then the code.

## Red flags

- "We might need this later": defer it.
- "This touches every job": check the boundaries; maybe it belongs in `src/shared/` or `src/internal/`.
- "No way to test this": the design or the requirement is unclear.
