# Coding principles

These rules apply to every job container and to the agents that implement them.

## What a test proves

A test proves the correct result. It does not prove how the code is arranged.

- Do not add unit tests or integration tests whose only job is to lock an internal function, a call order, or a private type.
- Do add system tests. Several cases. Each case runs the finished job container. The outcome is fixed before the test is written: exit code, detail code, and the records that must exist afterwards.
- The input fixture is whatever that stage actually consumes. It is not the same object for every job.

| Job | Fixture |
|---|---|
| ingest | A faked HTTP API response (Binance klines). |
| calculation-technical-indicator | A Parquet file of closed 1-hour candles. |
| backtest / experiment | A seeded Parquet lake and an experiment definition. |

The test does not import the service's packages. It builds the image, starts the container, and reads the outputs. The implementation stays unknown to the test.

## Audit

After the system tests pass, an audit checks three things only:

- Edge cases named in the issue (empty range, already processed, still open, invalid parameters, retry exhausted).
- Performance: a repeated run does not fetch or recompute work that is already stored.
- Clean code: the container is a runner, the job is stateless, and the exit codes match the container contract.

## HTTP

When a service has an HTTP API, or depends on a fixed HTTP contract, that behaviour is checked with Bruno (or an equivalent collection of HTTP requests with saved examples). That collection is not a substitute for the container system test.

## Agents

One orchestrator per issue. The orchestrator does not implement the issue itself. It spawns the sub-agents the issue needs: implementation, system tests, and the audit. It collects their results and does not widen the issue.
