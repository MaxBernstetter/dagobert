# Clean code

Code is read far more often than it is written. Optimize for the reader.

## Naming and language

- Use the domain's own terms: `candle`, `indicator`, `signal`, `action`, `fill`, `playbook`, `experiment`, `instance`. Don't use synonyms.
- Names say what something is or does; no abbreviations except established ones (`id`, `url`, `kpi`, `ttl`).
- Booleans read as questions (`isSimulated`, `hasOpenPosition`).

## Functions and packages

- **Small and focused:** one level of abstraction per function. If you need "and" to describe it, split it.
- **Few parameters:** group related values into a struct instead of passing many.
- **No hidden side effects:** a function named `get…` doesn't write.
- **Pure logic is separable:** calculations (indicators, KPIs, sizing, cost models) take values in and return values out. I/O (Parquet, SQLite, HTTP, Docker) lives at the edges, in `store/`, `venue/`, `ingest/` and `runner/`.
- **Dependencies injected:** pass collaborators in (store, venue client, clock); don't construct them inside use cases.

## Errors

- Fail fast and explicitly: validate at the boundary (environment, playbook config against `schema/playbook.schema.json`) and trust data inside.
- Map errors to the job's exit code at the edge: `1` for failure, `2` for already-stored or aborted work. Do not invent other codes.
- Never swallow errors. Log once at the boundary with context, then return or handle.

## Comments and formatting

- Code explains *what*; comments explain *why* (decisions, domain rules, non-obvious constraints such as bar gaps or exchange holes).
- Formatting and vet are automatic (`gofmt`, `go vet`); style is not debated in reviews.

## Boy-scout rule

Leave touched code a little cleaner than you found it, but keep refactors in scope. Larger clean-ups get their own issue, and a refactor never shares a pull request with a behaviour change (see [01-task-management](../01-task-management.md)).
