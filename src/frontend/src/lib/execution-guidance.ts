import type { ActivityRow, LiveState } from "@/types/read";

function matchesLiveRow(row: ActivityRow, live: LiveState): boolean {
  return (
    live.candleClose > 0 &&
    live.candleClose === row.candleClose &&
    live.symbol === row.symbol
  );
}

/** Short explanation for the Execution column (and detail panel). */
export function rowExecutionHint(row: ActivityRow, live: LiveState): string | null {
  switch (row.kind) {
    case "fill":
      return null;
    case "rejection":
      return "The simulated venue rejected the order; see venue status.";
    case "no-trade":
      return null;
    case "pending":
      return "Order was created at the venue but is not filled yet. The next execution run polls history — no second create.";
    case "not-yet-filled":
      break;
    default:
      return null;
  }

  const match = matchesLiveRow(row, live);

  if (live.stage === "execution" && match) {
    return "Execution is running for this candle. Refresh in a few seconds.";
  }
  if ((live.stage === "calculation-signal" || live.stage === "strategy") && match) {
    return "Strategy is running. Execution is the next pipeline step.";
  }
  if (live.stage !== "" && match && live.stage !== "execution") {
    const label = live.stage;
    return `Pipeline is on “${label}”. Execution has not run for this candle yet.`;
  }
  if (
    live.lastStage === "execution" &&
    match &&
    live.lastExit != null &&
    live.lastExit !== 0 &&
    live.lastExit !== 2
  ) {
    return `Execution failed (process exit ${live.lastExit}). Nothing was stored in execution.sqlite — no order, fill, or rejection row. Check execution container logs for DETAIL=.`;
  }
  if (live.lastStage === "execution" && match && live.lastExit === 2) {
    // Exit 2 without a row is unusual (e.g. no action); still show generic hint below.
  }

  const decision = row.decision ?? "trade";
  if (decision === "no-trade") {
    return "Strategy chose no-trade but execution has not recorded it yet. If the pipeline finished, re-run the execution stage.";
  }

  return `Strategy recorded “${decision}” but execution has not stored an outcome. Until it does, no order is on file and balances may be stale. If Processing is idle and this row is old, execution likely failed or was skipped — check logs or re-run execution for this symbol and candle.`;
}

export function countAwaitingExecution(rows: ActivityRow[]): number {
  return rows.filter((r) => r.kind === "not-yet-filled").length;
}

export function strategyPageIntro(awaiting: number, live: LiveState): string {
  if (awaiting === 0) {
    return "Each row is one candle decision for that playbook, with execution outcome when stored.";
  }
  const pipeline =
    live.stage !== ""
      ? ` Processing is on ${live.stage} for ${live.symbol} (one run at a time).`
      : live.lastStage === "execution" && live.lastExit === 1
        ? " The last execution step failed — hover execution badges for hints."
        : "";
  return `${awaiting} row(s) still have no execution record.${pipeline} Normal while the pipeline is running; otherwise check execution logs (DETAIL=).`;
}

export function executionKindHelp(kind: ActivityRow["kind"]): string {
  switch (kind) {
    case "fill":
      return "Execution stored a simulated fill.";
    case "rejection":
      return "The simulated venue rejected the order; the error was stored.";
    case "no-trade":
      return "Strategy skipped trading; execution recorded no-trade without placing an order.";
    case "pending":
      return "Order exists at the venue but is not filled in our store yet; the next run polls history.";
    case "not-yet-filled":
      return "Strategy wrote a decision; execution has not stored an outcome yet.";
    default:
      return "";
  }
}
