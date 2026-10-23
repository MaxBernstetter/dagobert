export const STRATEGY_COLUMN_HELP = {
  candle:
    "3-hour bar close time (UTC) this decision belongs to. One row per playbook, symbol, and close.",
  symbol: "Instrument the strategy evaluated (e.g. BTCUSDT). Execution may trade the venue pair (BTCUSDC).",
  signal: "Layer-1 signal from calculation-signal (buy / sell / no-trade). Indicator inputs are on the Data tab.",
  signalRule: "Playbook rule id that produced the signal.",
  action: "Layer-2 sized market action (may differ from the signal after allocation).",
  actionRule: "Sizing or allocation note from calculation-market-action.",
  rule: "Playbook rule (legacy column).",
  decision: "Sized market action decision.",
  execution:
    "Whether the execution job stored an outcome. Hover a badge for this row. Not the same as Bybit’s transaction log.",
  venue:
    "Exchange order status after we called Bybit (e.g. Filled, Created). Empty when we never reached the venue.",
} as const;

export const STRATEGY_PAGE_HELP =
  "Each section is one playbook. Rows show signals, sized actions, and execution outcomes — not indicator KPIs (see Data). Processing runs one pipeline step at a time today.";
