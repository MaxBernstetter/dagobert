# Dagobert — Idea

Dagobert is a backtesting framework for systematic crypto swing trading. It takes real exchange candles, turns them into indicators, runs deterministic rule-based playbooks, and simulates the resulting orders with trading costs. Every decision is reproducible: the same data, playbook and settings give the same fills and KPIs.

## Scope

1. **Backtest engine.** Replay historical candles through the indicator and playbook pipeline, simulate fills with trading costs, and report results against buy-and-hold.
2. **Experiments.** Run thousands of strategy instances (parameter grids) in one go and compare them by KPI.

Dagobert does not place orders on any exchange. A paper-trading or live-trading mode was part of the first plan and was removed; it can be rebuilt on top of the same playbook code. Machine-learning entry and exit timing is out of scope.
