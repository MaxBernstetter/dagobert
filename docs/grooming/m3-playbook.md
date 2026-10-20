# M3 — One playbook

First technical overview. Not a design.

Source: `docs/IDEA.md` section 7 (M3), plus the slice rule that the playbook already encodes size and exit. Decisions already made are restated below and are not reopened here.

## Service boundary

The strategy service is its own Go service and container, separate from ingest, process, and execution.

It reads KPI rows only: one row per symbol for one closed 3-hour candle, written by the process service. It does not read Parquet, does not call Binance, and does not call an exchange.

It writes one action record per symbol for that candle. Execution (M4) is the service that sends an action to the Bybit EU demo. This service stops at the record.

It does not compute indicators. A new indicator is a new parameter object on the same general row, for example `{"type": "MovingAverage", "method": "sma", "time_frame_hr": 3}`, plus a note under `docs/indicators/`. It is not a new column name and not a new table.

## Playbook, schema, lint

One YAML file in the repo. One strategy. A closed vocabulary: named comparisons of KPI columns to literals, a fixed rule order, and a declared action (`buy`, `sell`, or `no-trade`) with size and exit when the action is a trade. No formulas, no loops, and no embedded code. Changing the strategy means editing that file inside the vocabulary. It does not mean adding an expression.

Lint is one JSON Schema.

- **Editor.** The file carries a `yaml-language-server` schema directive so the editor checks it as it is edited.
- **CI.** The same schema is applied to the file on every change.
- **Service startup.** The Go service loads that schema and rejects the file before it runs. A file that fails lint does not produce an action.

## Action record

One record per playbook, symbol, and candle close. A no-trade is a record, not an absence.

Fields:

| Field | Meaning |
|---|---|
| Playbook identity | Which file produced the record |
| Symbol | KPI symbol, e.g. `BTCUSDT` |
| Candle close | UTC close of the 3-hour candle |
| Decision | `buy`, `sell`, or `no-trade` |
| Size and exit | As declared by the matched rule; empty on `no-trade` |
| Rule | Which rule matched, so a later view can show why |
| Written at | When this service wrote the record |

Idempotency key: playbook + symbol + candle close. A second run for that key writes nothing. It does not overwrite and it does not append.

## One run, one closed candle

1. The playbook has already passed schema lint at startup.
2. The run is given one candle close.
3. For each symbol in the slice (`BTCUSDT`, `ETHUSDT`), it reads that symbol's KPI row for the close.
4. It walks the playbook rules in order and stops at the first match.
5. It writes one action, or one explicit no-trade, for that symbol.
6. If the idempotency key already exists, it leaves the existing record and continues to the next symbol.

The run does not look at other candle closes, portfolio state, or the exchange.

## Exit codes

The container exits with one general code. That is the only process status. The job also reports a detail code that M3 owns. The Lambda records both. Logs are not the outcome. Any other process status is a defect in the runner.

| Code | Outcome |
|---|---|
| 0 | Succeeded. |
| 1 | Failed. |
| 2 | Aborted. |

| Detail | General | Outcome |
|---|---|---|
| 0 | 0 | An action, or an explicit no-trade, was written on this run. |
| 1 | 2 | Aborted. That record is already stored for this playbook, symbol, and candle close. No second write. |
| 10 | 1 | The candle close or symbol is missing or invalid. |
| 20 | 2 | The KPI row for that candle is not stored yet. |
| 30 | 1 | Reading the KPI row or writing the action failed after the configured retries. |
| 40 | 1 | The playbook failed schema validation. |

## Persistence

M3 owns its own store and does not open another job's. Locally that is `strategy.sqlite`. On AWS it is one DynamoDB on-demand table. This store holds actions only. It is not a store of indicator values, and it is not split per indicator.

The KPI row lives in M2's store. M3 owns that row's schema: one general record per symbol and bar, indicator values inside the record. M2 writes it. M3 does not give each indicator its own table.

The runner only reads by key and writes if that key is absent. The key is playbook plus symbol plus candle close. A hit is exit `2` with detail `1`: no second action. The item is the action, or the explicit no-trade.

## Acceptance

Proved only by system tests on the finished strategy container. The tests do not import the Go packages. The fixture is a seeded KPI row in the database. The outcome is a fixed action.

- A matching playbook writes one `buy` or `sell` and exits `0` / detail `0`.
- A playbook that does not match writes one `no-trade` and exits `0` / detail `0`.
- The same candle again exits `2` / detail `1` and does not write a second action.
- A missing KPI row exits `2` / detail `20`.
- A playbook that fails the JSON Schema exits `1` / detail `40`.
- The playbook refers to indicator parameter objects, not to column nicknames.

## Open questions

- Which single FR-05 strategy is this file, and which KPI values must M2 already have written for it?
- What is the playbook identity in the idempotency key when the YAML changes: a declared version, or a hash of the file?
- A missing or warm-up KPI row: write `no-trade`, or fail the run?
