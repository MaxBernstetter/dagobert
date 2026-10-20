# Feature Specification — Crypto Swing-Trading Platform

**Version:** 0.2  
**Status:** Revised PoC specification  
**Deployment:** Local Docker Compose, AWS-ready architecture  
**Trading style:** Systematic swing trading  
**Primary objective:** A complete, functioning vertical slice from historical data and live market ingestion through strategy evaluation, simulated execution and dashboard visualization.

## 1. Product definition

Build a modular crypto trading platform that uses real exchange market data to identify trading opportunities, evaluate deterministic trading strategies, apply risk controls and execute trades through interchangeable execution adapters.

The PoC must provide a complete operational loop, including:

- Historical market data acquisition and versioned Parquet datasets.
- Continuous ingestion of new market data.
- Technical indicator and feature calculation.
- A strategy engine implementing established trading approaches.
- A paper-trading execution engine and exchange sandbox integration where available.
- A protected integration path to a real exchange account.
- A Next.js administration dashboard focused on trading operations and portfolio state.
- A deployment architecture that runs locally and can migrate to AWS.

ML-based discovery of optimal entry and exit timestamps is explicitly deferred to a later development phase.

## 2. Initial trading assumptions

| Parameter | Initial specification |
|---|---|
| Market | Crypto spot |
| Initial assets | BTC/USDT and ETH/USDT |
| Strategy candle timeframe | 3 hours |
| Research comparison | 1-hour and 4-hour candles |
| Expected holding period | Hours to several days |
| Historical lookback | Up to five years per asset, subject to availability |
| Historical format | Apache Parquet |
| Initial decision engine | Deterministic, rule-based strategies |
| Execution modes | Paper, exchange sandbox, live adapter |
| Default execution mode | Paper |
| Backend | Python and FastAPI |
| Frontend | Next.js, TypeScript |
| Local orchestration | Docker Compose |
| Cloud target | AWS |
| Initial trading instruments | Spot only; no leverage or derivatives |

The 3-hour timeframe is a strategic choice rather than an assumption that every exchange provides native 3-hour candles. Where unsupported, construct them from validated lower-timeframe candles.

## 3. Functional requirements

### FR-01 — Historical market data acquisition

**Objective:** Establish a reproducible research dataset before developing trading strategies.

Requirements:

- Download historical OHLCV candles from the selected exchange or an appropriate historical data provider.
- Support configurable asset lists and date ranges.
- Target the last five years of available data.
- Implement pagination, rate-limit handling, retries and resumable downloads.
- Normalize timestamps to UTC.
- Detect duplicate records, missing intervals and invalid OHLCV values.
- Record the source exchange, market symbol, interval, retrieval time and dataset version.
- Preserve the original timeframe data used to construct derived candles.

Five years of history is a target, not a guarantee. Each asset's listing date and the provider's historical retention determine the actual coverage.

Historical candle endpoints are often paginated and may return incomplete current candles, so the ingestion process must explicitly handle both behaviors. See the [CCXT API documentation](https://github.com/ccxt/ccxt/wiki/manual).

**Acceptance criteria**

- Historical data can be downloaded for each configured asset.
- Downloads can resume after failure without creating duplicate logical records.
- The resulting dataset has a documented coverage range and quality report.
- The pipeline can reconstruct 3-hour candles from the validated base dataset.

### FR-02 — Parquet data lake

**Objective:** Separate historical research data from operational trading state.

Use Parquet as the primary format for immutable or versioned historical market datasets.

Proposed layout:

```text
data/
  raw/
    exchange=binance/
      symbol=BTC_USDT/
        timeframe=1h/
          year=2022/
          year=2023/
          year=2024/
          year=2025/
          year=2026/

  derived/
    exchange=binance/
      symbol=BTC_USDT/
        timeframe=3h/
          ...

  features/
    feature_version=v1/
      symbol=BTC_USDT/
        timeframe=3h/
          ...

  manifests/
    datasets/
    validation_reports/
```

The layout is illustrative. The implementation should choose partition keys that support efficient queries without creating excessive small files.

Requirements:

- Use PyArrow or an equivalent Parquet-compatible library.
- Store timestamps in a consistent UTC representation.
- Define a stable schema for OHLCV data.
- Make ingestion idempotent.
- Use dataset manifests containing source, time range, schema version and validation results.
- Support reproducible feature generation from a specified dataset version.
- Keep mutable order, account and execution state in PostgreSQL rather than Parquet.
- Design storage access so local files can later be replaced or supplemented by Amazon S3.

**Acceptance criteria**

A developer can build or rebuild the historical research dataset from documented commands and inspect its coverage, schema and quality.

### FR-03 — Live market data ingestion

**Objective:** Keep the system synchronized with current market conditions.

Requirements:

- Subscribe to a supported exchange WebSocket feed or poll REST endpoints as appropriate.
- Persist newly completed candles.
- Track data freshness and connection status.
- Recover missed intervals after reconnecting.
- Prevent incomplete candles from triggering unintended decisions.
- Append validated data to the historical dataset through a controlled persistence process.

For the PoC, strategy evaluation should occur after a 3-hour candle closes. Intrabar execution may be considered later, but it is not required initially.

### FR-04 — Candle construction and feature engineering

**Objective:** Produce consistent market features for historical evaluation and live decisions.

Initial indicators:

- RSI.
- MACD.
- Simple and exponential moving averages.
- Bollinger Bands.
- ATR.
- Donchian Channels.
- Rolling returns and momentum.
- Realized volatility.
- Rolling volume statistics.
- Price distance from moving averages.

Requirements:

- Define and version indicator parameters.
- Construct 3-hour candles using a fixed UTC boundary convention.
- Handle warm-up periods and missing observations.
- Avoid look-ahead leakage.
- Use identical feature calculations in historical backtesting and live inference.
- Permit additional timeframes as research inputs without changing the primary execution interval.

### FR-05 — Deterministic strategy engine

**Objective:** Implement and compare established trading strategies before introducing ML-based timing.

The strategy engine must support multiple independent strategies behind a common interface.

#### Strategy A — Trend following

Use moving-average relationships and trend confirmation to identify directional markets.

Possible initial rules:
- Enter long when a faster moving average crosses above a slower moving average and trend confirmation passes.
- Exit when the trend reverses or a defined risk exit triggers.

Parameters must be configurable and evaluated on unseen data.

#### Strategy B — Donchian breakout

Identify breakouts from prior rolling price highs or lows.

Possible initial rules:
- Enter long when the completed candle closes above the previous N-candle high.
- Exit on a lower-channel breakout, trailing stop or other predefined exit condition.

The breakout threshold must exclude the current candle from the prior rolling window.

#### Strategy C — Momentum

Use returns across multiple lookback periods to identify sustained positive or negative momentum.

Possible initial rules:
- Require positive momentum over the selected horizon.
- Filter entries using a trend or volatility condition.
- Exit when momentum deteriorates or the strategy's risk rule triggers.

#### Strategy D — Mean reversion

Test RSI and Bollinger Band extremes in range-bound market regimes.

Possible initial rules:
- Enter when price reaches a statistically unusual deviation from its rolling mean.
- Require a range-bound regime filter.
- Exit when price reverts toward the mean or a protective stop is triggered.

Do not assume that an oversold reading implies a reversal. Mean reversion can fail during strong trends.

#### Strategy E — Portfolio and exit overlays

Provide reusable risk overlays rather than duplicating risk logic within each strategy.

Candidates include:
- ATR-based stop distance.
- Volatility-adjusted position sizing.
- Trailing stops.
- Maximum holding period.
- Signal expiry.
- Maximum exposure and turnover limits.

The exact initial strategy parameters must be established through research and chronological validation, not selected solely because they maximize historical profit.

**Strategy interface**

Each strategy should accept a timestamped market/feature snapshot and current portfolio state, and return a structured target-position proposal with a rationale.

Strategies must not submit exchange orders directly.

### FR-06 — Strategy evaluation and backtesting

**Objective:** Determine whether the implemented strategies have an economic edge.

Requirements:

- Simulate decisions using only information available at the decision timestamp.
- Execute decisions no earlier than the next realistically tradable point after the signal.
- Model fees, spread, slippage and order-size constraints.
- Support cash, holdings, position sizing and portfolio equity.
- Record trades, fills, holding periods and exit reasons.
- Compare each strategy against buy-and-hold and a simple cash benchmark.
- Use chronological train/validation/test splits for strategy research.
- Include walk-forward evaluation and multiple market regimes.
- Keep a final holdout period separate from strategy selection.

Report:
- Net cumulative return.
- Annualized return where appropriate.
- Maximum drawdown.
- Sharpe and Sortino ratios.
- Profit factor.
- Win rate and average win/loss.
- Turnover and transaction costs.
- Trade count and average holding period.
- Performance by market regime and test period.

Research should explicitly account for data-mining bias and trading costs. Published studies have found that technical-rule profitability depends on the market, period, trading costs and validation methodology; no candidate strategy should be presumed profitable.

### FR-07 — Two-phase decision engine

**Phase 1: Traditional strategy engine — PoC scope**

The deterministic strategy implementations described in FR-05 are operational and drive all PoC trading decisions.

The system must support selecting an active strategy, inspecting its configuration and reviewing its historical and paper-trading performance.

**Phase 2: ML-based timing and decision optimization — future scope**

A later iteration may train an ML model using historical features and strategy outcomes to estimate more favorable entry and exit times.

Potential research components:
- Historical feature dataset.
- Candidate entry and exit labels.
- Future-return and risk estimates.
- Model training and evaluation.
- Out-of-sample comparison against the traditional strategies.
- Model inference and controlled strategy integration.

Optimal timestamps derived retrospectively from future price movements are **research labels, not information available to a live trading bot**. They must not be used directly as live predictions or leaked into training features.

The PoC should preserve the interfaces needed for this future model but must not depend on ML training to execute trades.

### FR-08 — Risk management

**Objective:** Enforce capital and execution limits independently of strategy logic.

Initial configurable controls:
- Maximum position per asset.
- Maximum portfolio exposure.
- Maximum order notional.
- Minimum expected edge after estimated costs.
- Maximum acceptable spread and slippage.
- Daily loss threshold.
- Maximum drawdown threshold.
- Maximum number of open orders.
- Minimum order size and exchange precision.
- Trading halt and kill switch.

The risk engine receives proposed target positions and current portfolio state, validates the action and either approves it or rejects it with a reason.

A risk halt prevents new order submissions. Restart and recovery behavior must be explicit.

### FR-09 — Execution and account integration

**Objective:** Connect strategy decisions to a reliable execution lifecycle.

Provide three execution modes:

1. **Paper broker:** Internal virtual portfolio and simulated order fills.
2. **Exchange sandbox:** Provider-managed test account, where available.
3. **Live adapter:** Real exchange account connectivity and order submission, disabled by default.

Requirements:
- Validate order sizes, precision and available balances.
- Track order lifecycle, partial fills, cancellations and rejections.
- Persist order requests, provider IDs and fill records.
- Handle timeouts and retries without duplicate orders.
- Reconcile local state with the exchange after restart.
- Model execution fees and configurable spread/slippage in paper trading.
- Separate read-only account connectivity tests from order submission.
- Use least-privilege API credentials and disable withdrawals.

Live execution must require an explicit, protected configuration change. Valid API credentials alone must never activate live trading.

### FR-10 — Portfolio accounting

Maintain a consistent portfolio ledger for each execution mode.

Required information:
- Cash and holdings.
- Current market valuation.
- Open orders and positions.
- Realized and unrealized P&L.
- Fees and execution costs.
- Equity curve and drawdown.
- Trade history and holding duration.
- Reconciliation discrepancies.

Every portfolio change must be attributable to an execution event or an explicit reconciliation adjustment.

### FR-11 — Next.js trading operations dashboard

**Objective:** Provide an operational view of the running trading system, not a generic task-management interface.

**Core screens**

1. **Trading overview**
   - Portfolio equity and P&L.
   - Active execution mode.
   - Current positions and exposures.
   - Recent strategy decisions.
   - Current market regime and data freshness.
   - System health and active risk alerts.

2. **Markets**
   - Candlestick charts.
   - Technical indicators.
   - Latest market price and volume.
   - Active strategy signals.
   - Historical signal markers where available.

3. **Strategies**
   - Enabled strategies.
   - Configured parameters.
   - Current target positions.
   - Recent entry/exit decisions.
   - Backtest and paper-trading performance comparisons.

4. **Portfolio and trades**
   - Current holdings.
   - Open orders.
   - Order and fill history.
   - Realized/unrealized P&L.
   - Fees, holding periods and exit reasons.

5. **Performance analytics**
   - Equity curve.
   - Drawdown chart.
   - Return and risk metrics.
   - Strategy comparisons.
   - Results by time period and market regime.

6. **System health**
   - Data ingestion status.
   - Last completed candle.
   - Feature-processing and strategy-evaluation status.
   - Exchange connectivity.
   - Execution errors and reconciliation status.

7. **Configuration and safety**
   - Selected symbols and strategy.
   - Paper/sandbox/live mode indicator.
   - Configured risk limits.
   - Trading halt.
   - Protected execution configuration.

The dashboard should emphasize financial state, market context, decision traceability and operational safety. Background jobs and data pipelines may be inspected as supporting system information, but must not dominate the interface.

### FR-12 — API and frontend integration

Use a documented backend API for the Next.js application.

Requirements:
- Read endpoints for market data, signals, strategies, portfolio, orders, fills and performance.
- Protected endpoints for strategy configuration and operational controls.
- Pagination for trade and event history.
- Consistent timestamps and monetary units.
- Clear separation between paper and real-account data.
- Authentication and authorization for privileged actions.
- A server-side integration path so exchange secrets never reach the browser.

WebSocket or server-sent-event updates may be added for live status and order events. Periodic polling is acceptable for the first PoC if it keeps the implementation simpler.

### FR-13 — Local deployment and AWS readiness

Run the complete platform with Docker Compose.

Initial logical components:
- Python backend and API.
- Market data ingestion.
- Strategy and feature processing.
- Training/backtesting commands.
- PostgreSQL.
- Next.js frontend.
- Optional event broker when asynchronous workloads justify it.

Use persistent volumes for PostgreSQL and the local Parquet data lake. Keep service configuration, credentials and environment-specific settings outside container images.

**AWS deployment mapping**

| Component | Potential AWS target |
|---|---|
| Python services | ECS with Fargate |
| Next.js frontend | Container deployment or an appropriate managed web-hosting service |
| PostgreSQL | Amazon RDS |
| Parquet datasets | Amazon S3 |
| Secrets | AWS Secrets Manager |
| Configuration | Systems Manager Parameter Store |
| Container images | Amazon ECR |
| Logs and metrics | Amazon CloudWatch |
| Scheduled backfills and training | EventBridge with container tasks |
| Event messaging | SQS or another appropriate service if required |
| Infrastructure | Terraform or AWS CDK |

Do not introduce a distributed broker, Kubernetes or multi-region architecture solely for the PoC. Preserve clean service interfaces and durable state so that deployment topology can evolve when actual scaling needs are known.

## 4. Suggested repository structure

```text
crypto-trading-platform/
  apps/
    api/                    # Python API
    dashboard/              # Next.js + TypeScript

  services/
    market_data/             # Historical and live ingestion
    features/                # Indicators and feature calculations
    strategies/              # Deterministic strategy implementations
    risk/                    # Risk validation and sizing
    execution/               # Paper, sandbox and live adapters
    portfolio/               # Ledger and P&L accounting

  research/
    notebooks/
    backtests/
    reports/

  data/
    raw/
    derived/
    features/
    manifests/

  models/                    # Versioned artifacts or artifact references

  tests/
    unit/
    integration/
    end_to_end/

  infra/
    docker/
    aws/
    terraform/

  docs/
    architecture/
    operations/
    strategy-research/

  docker-compose.yml
  .env.example
  README.md
```

This is a logical organization; the initial implementation may deploy several modules together in fewer containers.

## 5. Non-functional requirements

**Reliability**
- Recover from interrupted ingestion and exchange connectivity.
- Make processing idempotent.
- Preserve execution state across restarts.
- Halt trading when required data or account state is unreliable.

**Security**
- Never commit credentials.
- Separate paper and live configuration.
- Restrict exchange permissions and disable withdrawals.
- Protect administrative endpoints.
- Audit execution-mode changes.

**Reproducibility**
- Version datasets, features, strategies and configurations.
- Record the exact strategy version and configuration for every decision.
- Reproduce backtests from documented inputs.

**Financial integrity**
- Include trading costs.
- Separate simulated and actual returns.
- Avoid claims of guaranteed or consistent monthly returns.
- Maintain a traceable portfolio ledger.

**Maintainability**
- Automated tests and database migrations.
- Typed interfaces and documented contracts.
- Reproducible Docker builds.
- Explicit operational runbooks.

## 6. PoC acceptance criteria

The vertical slice is one closed 3-hour candle for `BTCUSDT` and `ETHUSDT`, from raw candles to one paper fill. It is complete when:

- [ ] Ingest and process are separate services, each its own container, locally (Floci) and on AWS. A Lambda starts the container for the service that is due. It does not run both stages in one process.
- [ ] Ingestion writes closed Binance 1-hour klines to Parquet, including `USDCUSDT`.
- [ ] The processor writes one KPI row per symbol for that 3-hour close.
- [ ] One YAML playbook, checked against its JSON Schema, emits one action order.
- [ ] The execution command submits that order to the Bybit EU spot demo (`BTCUSDC` / `ETHUSDC`) and records the fill or the rejection.
- [ ] Running the same candle close again does not write a second order.
- [ ] The Next.js app shows the latest ingestion and processing runs, the active playbook and its decisions, and the paper account's balance, orders, and fills.

Out of this slice: a second playbook, a backtester, five-year research, live trading, and a risk engine beyond what the playbook already encodes (size and exit).

## 7. Delivery milestones

**M1 — Ingest.** Its own Go service and container. Closed Binance 1-hour klines, including `USDCUSDT`, are written to Parquet. It does not compute KPIs and it does not write the database. Floci provides S3 locally.

**M2 — Process.** A different Go service and container. It reads the Parquet from M1 and writes one KPI row per symbol for that 3-hour close. It does not call Binance and it does not place orders. Floci provides DynamoDB locally.

**M3 — One playbook.** The strategy service reads only those KPI rows. JSON Schema lint, one YAML strategy, one action order for that candle. No exchange call yet.

**M4 — Paper fill.** The execution service sends the action to the Bybit EU demo and stores the fill. A repeat of the same candle close is a no-op.

**M5 — Scheduled run.** The 3-hour schedule runs the services in order: ingest, process, strategy, execution. Locally Floci does that. On AWS the schedule invokes a Lambda, and that Lambda starts the container for the service that is due. The Lambda does not run the trading steps itself.

**M6 — Frontend.** A Next.js app with three views:

- **Pipeline.** What an Airflow UI would show for this graph: each service run for a candle close, its status, start and finish, retries, and the error when it failed. This reads the Step Functions execution history. It does not add Airflow.
- **Strategies.** The active playbook, its rules, and the action it emitted for each candle, including an explicit no-trade.
- **Account.** Paper balance, open and recent orders, and fills from the Bybit EU demo ledger.

After the slice, and not part of it: backfill of older candles, a second playbook, a backtest against buy-and-hold, and any live-trading switch.

## 8. Research references

- Deprez and Frömmel, *Are simple technical trading rules profitable in bitcoin markets?* (2024). Research covering trading-rule selection, transaction costs and out-of-sample evaluation: https://doi.org/10.1016/j.irev.2024.05.003
- *Technical analysis in cryptocurrency markets: Do transaction costs and bubbles matter?* (2022): https://doi.org/10.1016/j.intfin.2022.101601
- CCXT exchange integration and historical OHLCV pagination: https://github.com/ccxt/ccxt/wiki/manual

These sources motivate the evaluation methodology; they do not establish that any particular strategy will remain profitable.

## 9. Final implementation principle

The PoC should first prove that one closed candle can be ingested, reduced to a KPI row, turned into one playbook action, and filled on the paper venue without a duplicate order.

Only after that foundation is reliable should the project introduce ML-based entry/exit optimization or pursue more ambitious return targets.

The primary deliverable is a functioning, auditable swing-trading platform—not a claim of profitable trading performance.
