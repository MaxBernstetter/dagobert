# trading-copilot

A personal copilot for systematic crypto spot swing trades on 3-hour UTC candles. It loads public Binance klines, computes one indicator record per closed candle, runs a single YAML playbook, and sends the resulting action to the Bybit EU spot demo. A Next.js app shows balances, past decisions and why they fired, and the fill or rejection.

The pipeline is four stateless Go containers, in order:

1. **Ingest** writes closed 1-hour klines to Parquet.
2. **Process** writes one KPI record per symbol and 3-hour bar.
3. **Strategy** writes one action, including an explicit no-trade.
4. **Execution** places the paper order and stores the fill and the current balance.

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

`task up` builds the images and runs one candle window: ingest, process, strategy, then the Bybit EU demo order. The app stays on [http://localhost:3000](http://localhost:3000). Overview is the balances and the activity. Pipeline is the other route. `task down` stops the stack.

The default window is 2023-11-14 20:00–23:00 UTC (`FROM_MS` `1699995600000`, `TO_MS` and `CANDLE_CLOSE_MS` `1700006400000`, `BAR_OPEN_MS` `1699995600000`, `SYMBOL` `BTCUSDT`). Export a different value before `task up` to change it. A second `task up` for the same window does not place a second order.

Ingest also accepts `ETHUSDT` and `USDCUSDT`. Process, strategy, and execution accept `BTCUSDT` and `ETHUSDT`. Execution trades `BTCUSDC` and `ETHUSDC` on `https://api-demo.bybit.eu`.

## Development

Workflow, GitHub issues, and the agent harness live in [docs/development](docs/development/README.md). Milestone notes are in [docs/grooming](docs/grooming). Venue and kline contracts are in [docs/ingest/binance-public-klines.md](docs/ingest/binance-public-klines.md) and [docs/execution/bybit-eu-demo.md](docs/execution/bybit-eu-demo.md).

`devbox shell` installs Task (`go-task`). From that shell:

```bash
task build   # images only
task test    # system tests for the four job containers
task up      # one candle window and the app
task down
```

The system tests build each job image and run it. They do not import the service packages. Go 1.23 must be on `PATH` for `task test`. `npm run build` in `web/` typechecks the app. There is no browser test suite.
