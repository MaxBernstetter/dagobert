# Dagobert — Idea

Dagobert is a systematic crypto swing-trading platform. It takes real exchange candles, turns them into indicators, runs deterministic rule-based playbooks, and places the resulting orders on a simulated venue. Every candle close is processed once, every decision is recorded with the exact playbook version that made it, and the same playbook code runs in backtests, paper trading, and later live trading.

## Roadmap

1. **Backtest Execution Engine.** Replay historical candles through the indicator and playbook pipeline, simulate fills with trading costs, and report results against buy-and-hold.
2. **Papertrading Simulator.** Run the same pipeline on closed candles in near real time and fill orders on the Bybit EU demo venue, with balances, orders and fills tracked.
3. **Live Trading Bot.** Switch the execution adapter to a real account behind explicit configuration, with restricted API permissions, withdrawals disabled, and trading halted when data or account state is unreliable.

Each step has to be proven before the next starts. Machine-learning entry and exit timing is out of scope until the roadmap is complete.
