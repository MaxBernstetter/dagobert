# Dagobert

Personal experiment suite for systematic crypto spot swing trades on 3-hour UTC candles. It loads public Binance klines, computes one indicator record per closed candle, runs Go playbook strategies, and can send actions to the Bybit EU spot demo. The web UI covers experiments, providers, and experiment results.

The orchestrator is the long-running process. The pipeline is five stateless Go containers, started by the orchestrator when a candle closes:

1. **Ingest** writes closed 1-hour klines to Parquet.
2. **Calculation (technical indicator)** writes one KPI record per symbol and 3-hour bar (`technical_indicator.sqlite`).
3. **Calculation (signal)** writes one **signal** per candle (buy / sell / no-trade; no size).
4. **Calculation (market action)** reads the signal and **live Bybit balances**, then writes the sized **action**.
5. **Execution** places the paper order and stores the fill and balance snapshot.

Each container prints `DETAIL=<n>` and exits `0` (succeeded), `1` (failed), or `2` (aborted). A repeat of work that is already stored exits `2`.

## Run it

You need Docker and [Devbox](https://www.jetify.com/devbox).

Bybit credentials are a demo key on a Bybit EU account. Public Binance klines need no key.

1. Register at [bybit.eu](https://www.bybit.eu). API key creation can take up to 48 hours after the account is opened. Turn on Google Authenticator.
2. Open Demo Trading in that account and create the API key there. The demo user is separate from the live user, so a live key is rejected by the demo host.
3. Enable trade permission. Leave withdrawals off.
4. Put the key and secret in a gitignored `.secrets` file in the repo root:

```bash
BYBIT_API_KEY=...
BYBIT_API_SECRET=...
```

Compose gives that file to the execution container only. The web app never reads it. The key is accepted only on `https://api-demo.bybit.eu`.

```bash
devbox shell
task up
```

Run `task build` once (job images + stack), then `task up` for Floci, the orchestrator, and the production frontend at [http://localhost:3001](http://localhost:3001). For UI work use `task dev` instead: Compose **watch** syncs `./src/frontend` into the container and runs the Vite dev server (no full image rebuild on each save). `task up:build` rebuilds everything then starts the stack. Set `WEB_PORT` (default 3001), `FLOCI_PORT` (default 4566), `POSTGRES_PORT` (default 5432), or `ORCHESTRATOR_PORT` (default 8090) in `.env` to publish those services on other host ports. The four jobs are not part of that stack. The orchestrator starts them when a 3-hour UTC candle closes. `task run` starts one execution now. `CANDLE_CLOSE_MS` defaults to `1700006400000` (2023-11-14 23:00 UTC). A second `task run` for the same close does not place a second order. The overview can also backfill hourly candles, or recalculate indicators, for a UTC from/to range. Those two steps do not place orders. `task down` stops the stack and leaves `data/` in place.

Ingest reads public klines from `https://data-api.binance.vision` unless `BINANCE_BASE_URL` is set. It also accepts `ETHUSDT` and `USDCUSDT`. Process, strategy, and execution accept `BTCUSDT` and `ETHUSDT`. Execution trades `BTCUSDC` and `ETHUSDC` on `https://api-demo.bybit.eu`.

## Development

Workflow, GitHub issues, and the agent harness live in [docs/development](docs/development/README.md). How the local stack runs is in [docs/architecture/local-stack.md](docs/architecture/local-stack.md). Indicator and playbook documentation for the Wiki is described in [docs/architecture/wiki-catalog.md](docs/architecture/wiki-catalog.md). Closed milestone notes are in [docs/grooming/archive](docs/grooming/archive). Venue and kline contracts are in [docs/ingest/binance-public-klines.md](docs/ingest/binance-public-klines.md) and [docs/execution/bybit-eu-demo.md](docs/execution/bybit-eu-demo.md).

`devbox shell` installs Go 1.24 and Task (`go-task`). From that shell:

```bash
task build      # images only
task test       # system tests for the four job containers
task up         # stack (after task build)
task dev        # compose up --watch (optional; task up already bind-mounts the frontend)
task up:prod    # production frontend image (no override)
task up:build   # rebuild all images, then stack
task run        # one candle close, CANDLE_CLOSE_MS optional
task down
```

The system tests build each job image and run it. They do not import the service packages. Go 1.24 must be on `PATH` for `task test`. Application code lives under `src/` (jobs, `shared/`, `internal/instruments`, and `frontend/`, the Vite SPA). `npm run build` in `src/frontend/` typechecks the app. There is no browser test suite.
