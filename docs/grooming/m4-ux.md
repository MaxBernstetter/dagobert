# M4 — What the trader sees

Audit of the paper-fill job against a person who wants an overview and a detail view. Not a component spec. No browser test runner. The execution container stays the only thing that talks to Bybit.

## Required features

| Need | Overview | Detail |
|---|---|---|
| Trading activity | Last action for the latest candle: symbol, buy / sell / no-trade, filled or rejected | One row per candle: side, USDC spent, base filled, average price, order status |
| Past activity and reasoning | The rule name that fired, next to that last action | The playbook, the matched rule, and the indicator comparison that caused it, beside the fill or the rejection |
| Current balance | One figure per coin that has a `cashBalance` | That coin's latest `cashBalance`, the time of the movement, and the order that changed it when there was one |

The browser does not compute these. It reads stored records.

## What M4 stores today, and what is missing

The execution note stores one item per `orderLinkId`: the fill fields, or `retCode` and `retMsg`, or a settled no-trade. That is enough for a fill line. It is not enough for the three needs above.

| Need | Already stored | Missing |
|---|---|---|
| Activity line | Fill fields on the execution item | A stable link from that item to the candle close and the symbol the playbook used (`BTCUSDT`) as well as the venue symbol (`BTCUSDC`) |
| Reasoning | The action row in the strategy store: decision, rule, size, exit | Nothing in the execution item. The screen joins the action on playbook + symbol + candle close. M4 does not copy the rule |
| Balance | Described as a transaction-log read inside the job | Not written into `execution.sqlite`. The overview has nothing to read after the container exits |

M4 must write a balance snapshot on every run that reaches Bybit or that settles a no-trade: one row per currency, `cashBalance`, `transactionTime`. Coins with no log row stay absent. The snapshot is replaced, not appended. History of balances is the activity list, not a second ledger.

## Navigation

Two levels. No settings, no order ticket, no pipeline graph.

1. **Overview.** The first screen. Balances across the top. Under them, the latest candle: decision, rule name, and whether it filled. A row that is not filled yet, or was rejected, is visible here.
2. **Activity.** A list of past candles, newest first. Each row is one candle, including an explicit no-trade.
3. **Detail.** Opens from a row. It does not navigate to a new product area. It shows the reasoning and the fill for that candle, and the balance movement tied to that order when there was one.

Back returns to the list with the same scroll position. The overview is the list with the latest row expanded.

## Layout

One column on a narrow window, the list beside the detail on a wide window. Balances stay above the list in both.

Use ready-made components. Do not write a button, a table, a card, or a badge. For this Next.js app that means [shadcn/ui](https://ui.shadcn.com/) on Tailwind: `Card` for each balance, `Table` for the activity, `Badge` for filled, rejected, no-trade, and not-yet-filled, `Sheet` for the detail on a narrow window and a second column on a wide one.

No chart library in this pass. Reasoning is the rule text and the comparison values already on the action, not a price graph.

## What this screen does not do

- No Cypress and no Playwright.
- No control that places, cancels, or amends an order.
- No exchange secret in the browser.
- No pipeline view and no indicator editor. Those stay on the later frontend milestone.

The Next.js server reads the SQLite files. There is no separate read API. A rejection is shown as `retMsg` on the row. App structure is [m6-frontend.md](m6-frontend.md). The job that writes these records is [m4-paper-fill.md](m4-paper-fill.md).
