# M6 — Frontend

Next.js app. Trader navigation and the look of the account screens are [m4-ux.md](m4-ux.md). This note is the app structure around that flow.

No Cypress and no Playwright. Ready-made components only: [shadcn/ui](https://ui.shadcn.com/) on Tailwind. Do not write a button, table, card, badge, sheet, or navigation item.

## What the person does

Home is the overview: balances, then the latest candle.

The activity list is the same page, older candles under the latest one. A row opens the detail beside the list on a wide window and in a `Sheet` on a narrow one. Detail shows the playbook, the matched rule, the decision, and the fill, rejection, or no-trade. Back keeps the list where it was.

A second route, Pipeline, shows one row per service for a candle close. It is not the home screen. There is no order ticket and no control that places or cancels an order.

## Routes

| Path | Screen |
|---|---|
| `/` | Overview and activity. Balances, latest candle, older candles. |
| `/pipeline` | Ingest, process, strategy, execution for one candle close. |

Detail is not a third product area. It is the selected row on `/`.

## Where the data comes from

The browser calls this app only. The Next.js server reads the stores. There is no separate read API in this slice.

| Screen | Store | Record |
|---|---|---|
| Balances | `execution.sqlite` | Replaced `cashBalance` per currency |
| Activity and fill | `execution.sqlite` | One item per `orderLinkId` |
| Rule and decision | `strategy.sqlite` | Action body: `decision`, `rule`, `size`, `exit` |
| Pipeline | Step Functions history (Floci locally) | Status, start, finish, retries, error |

Join activity to reasoning on playbook + symbol + candle close. The execution item does not copy the rule. A rejected row shows `retMsg` as the status line. The overview does not render the raw Bybit body.

`DATA_DIR` is the server environment. `BYBIT_API_KEY` and `BYBIT_API_SECRET` are not in this app.

## Structure

```text
web/
  app/layout.tsx          shell: wordmark, Overview, Pipeline
  app/page.tsx            overview + activity, server component
  app/pipeline/page.tsx   service rows for one candle close
  lib/read.ts             sqlite reads and the join
  components/ui/          shadcn only
```

`Card` for each balance. `Table` for activity and for pipeline. `Badge` for filled, rejected, no-trade, and not-yet-filled. `Sheet` for detail on a narrow window. The wide layout is a two-column grid from Tailwind, list and detail.

Server components read on each request. No client cache of balances. No chart library.

## What the app does not do

- No trading buttons. Orders stay on the execution service.
- No exchange secret in the browser.
- No second pipeline store beside Step Functions history.
- No second playbook, no backtest, no KPI charts, no live-trading switch.

## Acceptance

The app is reviewed by opening it. There is no browser test suite in this slice.

| Check | Result |
|---|---|
| Seeded fill, action, and balance rows | Overview shows the coins, the latest decision, the rule name, and the fill. |
| A no-trade row | The list includes it. Detail shows the rule and no order id. |
| A rejection | The row shows `retMsg`. No fill numbers. |
| Pipeline route | Four service rows for one candle close, from history. |
| Page source and network | No Bybit host and no API secret. |
