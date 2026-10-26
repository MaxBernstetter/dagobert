# Documentation map

One line per document, grouped by purpose. Current state and next steps: [STATUS.md](STATUS.md). Terms: [glossary.md](glossary.md).

## Start here

- [../README.md](../README.md): what Dagobert is and how to run it.
- [IDEA.md](IDEA.md): the original idea and roadmap.
- [STATUS.md](STATUS.md): what works, what is not done, next steps, how to resume after a break.
- [glossary.md](glossary.md): project terms with the code location of each.
- [../CLAUDE.md](../CLAUDE.md): commands, architecture and conventions for agents.
- [../AGENTS.md](../AGENTS.md): workflow rules (issues, project board, worktrees, pull requests).

## Architecture

- [architecture/local-stack.md](architecture/local-stack.md): the local Docker Compose stack and what it persists.
- [architecture/pipeline-data-flow.md](architecture/pipeline-data-flow.md): the live five-job pipeline step by step.
- [architecture/experiment-data-flow.md](architecture/experiment-data-flow.md): experiment and instance flow from the API to the stored result.
- [architecture/backtesting.md](architecture/backtesting.md): the experiment engine (Go and C++), phases and persistence.
- [architecture/calculation-market-action.md](architecture/calculation-market-action.md): how the signal is sized against live balances.
- [architecture/wiki-catalog.md](architecture/wiki-catalog.md): how indicator and playbook docs feed the frontend Wiki.

## Features and specs

- [v2/features/ingestion.md](v2/features/ingestion.md): ingestion feature spec.
- [v2/features/position-sizing.md](v2/features/position-sizing.md): position sizing method (unit risk, heat, max units); test vectors in [v2/features/position-sizing-vectors.json](v2/features/position-sizing-vectors.json).
- [v2/features/backtesting.md](v2/features/backtesting.md): backtesting feature spec (overlaps architecture/backtesting.md).
- Playbooks: [ma-cross](../src/calculation-signal/playbooks/ma-cross/DESCRIPTION.md), [signal-consensus](../src/calculation-signal/playbooks/signal-consensus/DESCRIPTION.md), [position-sizing](../src/calculation-signal/playbooks/position-sizing/DESCRIPTION.md).
- Strategy KPIs: [active-return](../src/calculation-signal/playbooks/strategy-kpi/active-return/README.md), [cagr](../src/calculation-signal/playbooks/strategy-kpi/cagr/README.md), [max-drawdown](../src/calculation-signal/playbooks/strategy-kpi/max-drawdown/README.md), [sharpe](../src/calculation-signal/playbooks/strategy-kpi/sharpe/README.md), [sortino](../src/calculation-signal/playbooks/strategy-kpi/sortino/README.md), [win-rate](../src/calculation-signal/playbooks/strategy-kpi/win-rate/README.md).

## Development process

Index: [development/README.md](development/README.md).

- [development/01-task-management.md](development/01-task-management.md): issues, project board, statuses.
- [development/02-github-mcp.md](development/02-github-mcp.md): GitHub access through the MCP server.
- [development/03-agent-harness.md](development/03-agent-harness.md): which model does which work.
- [development/04-coding-principles.md](development/04-coding-principles.md): job containers, system tests, audit.
- [development/principles/README.md](development/principles/README.md): planning and implementation principles and the Definition of done.
- [development/05-cpp-benchmark.md](development/05-cpp-benchmark.md): C++ versus Go benchmark and Postgres persistence cost.
- [development/protobuf.md](development/protobuf.md): protobuf and Buf contracts.
- [development/run-backtest.md](development/run-backtest.md): run a backtest or experiment locally.
- [development/seed-demo-year.md](development/seed-demo-year.md): seed one year of demo history.
- [../tools/claude-plugin/README.md](../tools/claude-plugin/README.md): the agent plugin (groomer, planner, validator, worker).

## Contracts

- Ingest: [ingest/binance-public-klines.md](ingest/binance-public-klines.md) (Binance klines, decimal candle schema).
- Execution: [execution/bybit-eu-demo.md](execution/bybit-eu-demo.md) (Bybit EU demo venue).
- Indicators: [indicators/schema.md](indicators/schema.md) plus one page per indicator: [atr](indicators/atr.md), [bollinger](indicators/bollinger.md), [close-distance](indicators/close-distance.md), [donchian](indicators/donchian.md), [ema](indicators/ema.md), [macd](indicators/macd.md), [realized-volatility](indicators/realized-volatility.md), [returns](indicators/returns.md), [rsi](indicators/rsi.md), [sma](indicators/sma.md), [volume](indicators/volume.md).
- Infrastructure: [../infra/terraform/README.md](../infra/terraform/README.md) (Floci and AWS, Supabase).

## Design notes (docs/grooming)

Written before or during a change. They explain why, and can be out of date; the architecture docs and the code win.

Current:

- [serverless-aws.md](grooming/serverless-aws.md): AWS plan with a status ladder (steps 0-14); the closest thing to a roadmap.
- [experiment-api-scale.md](grooming/experiment-api-scale.md): scaling the Experiment API.
- [experiments-ready-strategies.md](grooming/experiments-ready-strategies.md): strategies ready for experiments.
- [signal-consensus-playbook.md](grooming/signal-consensus-playbook.md): design of the signal-consensus playbook.
- [atlas-schema.md](grooming/atlas-schema.md): Atlas schema and migrations.
- [ti-parquet-cache.md](grooming/ti-parquet-cache.md): technical-indicator Parquet cache.
- [ingest-provider-facade.md](grooming/ingest-provider-facade.md), [ingest-fetch-facade-refactor.md](grooming/ingest-fetch-facade-refactor.md), [ingest-list-assets.md](grooming/ingest-list-assets.md), [ingestion-providers-ui.md](grooming/ingestion-providers-ui.md): ingest provider design.
- [saxo-openapi-evaluation.md](grooming/saxo-openapi-evaluation.md): evaluation of the Saxo OpenAPI.

Superseded or archived (kept for history):

- [backtesting.md](grooming/backtesting.md): superseded by architecture/backtesting.md.
- [deploy-supabase.md](grooming/deploy-supabase.md): early deploy plan, older vocabulary.
- [postgres-control-plane.md](grooming/postgres-control-plane.md): redirect stub.
- [grooming/archive/](grooming/archive/): closed milestones m1-ingest, m2-process, m3-playbook, m4-paper-fill, m4-ux, m5-schedule, m6-frontend.

## Per-package READMEs

- [src/backtest/README.md](../src/backtest/README.md): experiment and backtest engine.
- [src/experimentapi/README.md](../src/experimentapi/README.md): Experiment API.
- [src/orchestrator/README.md](../src/orchestrator/README.md): orchestrator and starter.
- [src/calculation/README.md](../src/calculation/README.md): C++ calculation engine and builds.
- [src/frontend/README.md](../src/frontend/README.md): Vite + React frontend.
- [src/calculation-technical-indicator/internal/indicators/](../src/calculation-technical-indicator/internal/indicators/): one README per indicator package (atr, close-distance, exponential-moving-average, macd, rsi, simple-moving-average, volume-mean).
- [tests/README.md](../tests/README.md): system tests.
- [db/README.md](../db/README.md): schema, migrations, sqlc.
- [proto/README.md](../proto/README.md): protobuf contracts.
