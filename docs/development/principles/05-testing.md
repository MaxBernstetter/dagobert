# Testing

A piece of software is working when its **acceptance criteria are proven by automated tests** and its **behaviour is observable in operation**.

The job-container rules in [04-coding-principles](../04-coding-principles.md) define how jobs are proven: system tests that run the built image against a fixed fixture and check exit code, `DETAIL`, and stored records. The rules below add to them.

## Start from the acceptance criteria

1. Turn each acceptance criterion of the issue into at least one test before or while implementing.
2. If a criterion can't be tested, clarify it in the issue first.
3. Test behaviour through the public interface (the container, or an exported function of a pure package), not its internals.

## Test levels

| Level | Proves | Where | Run with |
|---|---|---|---|
| Unit | Pure logic is correct (indicator math, KPIs, sizing, cost model, playbook decisions) | `*_test.go` next to pure packages (`src/internal/money`, `src/internal/venue/sim`, `src/calculation-signal/playbooks/*`) | `go test ./src/...` |
| System | A job image does its stage correctly, end to end | `tests/system/<job>/` | `task test`, or `go test ./tests/system/<job> -run <Name> -count=1` after `task build:jobs` |
| Frontend | The app typechecks and builds | `src/frontend` | `npm run build` |

There is no browser or contract test suite yet. Do not add one as a side effect of another change.

## Domain-specific test rules

- **Idempotency:** every job has a case showing that a second run on the same input exits `2` (or `DETAIL=1`) and writes nothing new.
- **Gaps and holes:** a case covers missing exchange hours. They are recorded as `unavailable`, never synthesized, and the next stage must treat them correctly.
- **Reference values:** numerical code (indicators, KPIs, costs) is checked against known values with explicit tolerances.
- **Deterministic data:** tests use fixed fixtures and fixed candle close times. No dependency on today's date or live sources.
- **Bugs:** a fix starts with a failing test that reproduces the bug.

## Validation beyond tests

Tests prove correctness before a change ships. Operation proves it after:

- After a run, `task run` for the new close shows the action and, when an order is sent, the fill and balances in the app.
- A rerun of the same close does not place a second order.

## Done means

See the [Definition of done](README.md#definition-of-done).
