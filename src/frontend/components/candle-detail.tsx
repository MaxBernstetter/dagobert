import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import type { ActivityRow, LiveState } from "@/types/read";
import {
  executionKindHelp,
  rowExecutionHint,
} from "@/lib/execution-guidance";
import { formatPlaybookTitle } from "@/lib/strategy-groups";
import { STRATEGY_COLUMN_HELP } from "@/lib/strategy-help";
import { HelpTip } from "@/components/help-tip";
import { StatusBadge, rowVenueStatus, venueStatusAddsDetail } from "@/components/status-badge";

function formatCandleClose(ms: number): string {
  if (!ms) return "—";
  return new Date(ms).toISOString().replace("T", " ").replace(/\.\d{3}Z$/, " UTC");
}

function PopupField({ label, value }: { label: string; value: string }) {
  return (
    <span className="inline-flex items-baseline gap-1.5">
      <span className="text-muted-foreground">{label}</span>
      <span>{value}</span>
    </span>
  );
}

function sameWord(a: string | null | undefined, b: string | null | undefined): boolean {
  const left = (a ?? "").trim().toLowerCase();
  return left !== "" && left === (b ?? "").trim().toLowerCase();
}

function ActionPopup({
  row,
  live,
  onClose,
}: {
  row: ActivityRow;
  live: LiveState;
  onClose?: () => void;
}) {
  const hint = rowExecutionHint(row, live);
  const execTip = [executionKindHelp(row.kind), hint].filter(Boolean).join(" ");
  const signalDecision = row.signalDecision?.trim() ?? "";
  const signalRule = row.signalRule?.trim() ?? "";
  const actionRule = row.rule?.trim() ?? "";
  const showSignalDecision = signalDecision !== "" && !sameWord(signalDecision, row.decision);
  const showSide = Boolean(row.side) && !sameWord(row.side, row.decision);
  const showFilledQty = Boolean(row.cumExecQty) && row.cumExecQty !== row.qty;

  return (
    <div className="rounded-lg border-2 border-sky-500 bg-card p-3 text-sm shadow-lg shadow-sky-500/20">
      <div className="flex items-center gap-2">
        <p className="min-w-0 flex-1 font-medium">
          {formatCandleClose(row.candleClose)}
          {row.decision ? (
            <>
              <span className="mx-1.5 text-muted-foreground">·</span>
              {row.decision}
            </>
          ) : null}
        </p>
        <span title={execTip || undefined} className="shrink-0 cursor-help">
          <StatusBadge kind={row.kind} />
        </span>
        {onClose && (
          <button
            type="button"
            aria-label="Close"
            className="shrink-0 rounded-md p-1 text-muted-foreground hover:bg-muted"
            onClick={onClose}
          >
            ×
          </button>
        )}
      </div>

      {venueStatusAddsDetail(row) && row.kind !== "rejection" && (
        <p className="mt-1 text-xs text-muted-foreground">
          <HelpTip label="Venue" tip={STRATEGY_COLUMN_HELP.venue} />
          <span className="ml-1.5 text-foreground">{rowVenueStatus(row)}</span>
        </p>
      )}

      {(showSignalDecision || signalRule || actionRule || row.size != null) && (
        <div className="mt-2 space-y-1 text-xs">
          {showSignalDecision && (
            <p>
              <HelpTip label="Signal" tip={STRATEGY_COLUMN_HELP.signal} />
              <span className="ml-1.5">{signalDecision}</span>
            </p>
          )}
          {signalRule && (
            <p>
              <HelpTip label="Signal rule" tip={STRATEGY_COLUMN_HELP.signalRule} />
              <span className="ml-1.5 font-mono break-words">{signalRule}</span>
            </p>
          )}
          {actionRule && (
            <p>
              <HelpTip label="Action rule" tip={STRATEGY_COLUMN_HELP.actionRule} />
              <span className="ml-1.5 font-mono break-words">{actionRule}</span>
            </p>
          )}
          {row.size != null && (
            <p>
              <span className="text-muted-foreground">Size</span>
              <span className="ml-1.5">{String(row.size)}</span>
            </p>
          )}
        </div>
      )}

      {row.kind === "fill" && (
        <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1 border-t pt-2 text-xs">
          {showSide && <PopupField label="Side" value={row.side ?? ""} />}
          {row.qty && <PopupField label="Qty" value={row.qty} />}
          {showFilledQty && <PopupField label="Base qty" value={row.cumExecQty ?? ""} />}
          {row.avgPrice && <PopupField label="Avg price" value={row.avgPrice} />}
          {row.cumExecValue && row.cumExecValue !== row.qty && (
            <PopupField label="Value" value={row.cumExecValue} />
          )}
        </div>
      )}
      {row.kind === "rejection" && row.retMsg && (
        <p className="mt-3 border-t pt-2 text-xs text-destructive">{row.retMsg}</p>
      )}
    </div>
  );
}

export function CandleDetail({
  row,
  live,
  variant = "card",
  onClose,
}: {
  row: ActivityRow | null;
  live: LiveState;
  variant?: "card" | "popup";
  onClose?: () => void;
}) {
  if (variant === "popup" && row) {
    return <ActionPopup row={row} live={live} onClose={onClose} />;
  }

  if (!row) {
    return (
      <Card className="h-full">
        <CardHeader>
          <CardTitle>Detail</CardTitle>
          <CardDescription>Select a row from the activity list.</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  const hint = rowExecutionHint(row, live);
  const execTip = [executionKindHelp(row.kind), hint].filter(Boolean).join(" ");

  return (
    <Card className="h-full">
      <CardHeader>
        <CardTitle className="text-lg">{row.symbol}</CardTitle>
        <CardDescription>
          {formatPlaybookTitle(row.playbook)} · {formatCandleClose(row.candleClose)}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 text-sm">
        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2">
          <dt className="text-muted-foreground">Playbook id</dt>
          <dd className="font-mono text-xs">{row.playbook || "—"}</dd>
        </dl>

        <div className="border-t pt-4 space-y-2">
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Signal</p>
          <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2">
            <dt className="text-muted-foreground">
              <HelpTip label="Decision" tip={STRATEGY_COLUMN_HELP.signal} />
            </dt>
            <dd>{row.signalDecision ?? "—"}</dd>
            <dt className="text-muted-foreground">
              <HelpTip label="Rule" tip={STRATEGY_COLUMN_HELP.signalRule} />
            </dt>
            <dd className="font-mono text-xs break-all">{row.signalRule ?? "—"}</dd>
          </dl>
        </div>

        <div className="border-t pt-4 space-y-2">
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Market action</p>
          <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2">
            <dt className="text-muted-foreground">
              <HelpTip label="Decision" tip={STRATEGY_COLUMN_HELP.action} />
            </dt>
            <dd>{row.decision ?? "—"}</dd>
            <dt className="text-muted-foreground">Size</dt>
            <dd>{row.size != null ? String(row.size) : "—"}</dd>
            <dt className="text-muted-foreground">
              <HelpTip label="Rule" tip={STRATEGY_COLUMN_HELP.actionRule} />
            </dt>
            <dd className="font-mono text-xs break-all">{row.rule ?? "—"}</dd>
          </dl>
        </div>

        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 border-t pt-4">
          <dt className="text-muted-foreground">
            <HelpTip label="Execution" tip={STRATEGY_COLUMN_HELP.execution} />
          </dt>
          <dd>
            <span title={execTip || undefined} className="cursor-help inline-flex">
              <StatusBadge kind={row.kind} />
            </span>
          </dd>
          <dt className="text-muted-foreground">
            <HelpTip label="Venue status" tip={STRATEGY_COLUMN_HELP.venue} />
          </dt>
          <dd>{rowVenueStatus(row)}</dd>
        </dl>

        {row.kind === "fill" && (
          <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 border-t pt-4">
            <dt className="text-muted-foreground">Side</dt>
            <dd>{row.side ?? "—"}</dd>
            <dt className="text-muted-foreground">Qty</dt>
            <dd>{row.qty ?? "—"}</dd>
            <dt className="text-muted-foreground">Filled qty</dt>
            <dd>{row.cumExecQty ?? "—"}</dd>
            <dt className="text-muted-foreground">Exec value</dt>
            <dd>{row.cumExecValue ?? "—"}</dd>
            <dt className="text-muted-foreground">Avg price</dt>
            <dd>{row.avgPrice ?? "—"}</dd>
            <dt className="text-muted-foreground">Market unit</dt>
            <dd>{row.marketUnit ?? "—"}</dd>
            {row.venueSymbol && (
              <>
                <dt className="text-muted-foreground">Venue symbol</dt>
                <dd>{row.venueSymbol}</dd>
              </>
            )}
          </dl>
        )}

        {row.kind === "rejection" && row.retMsg && (
          <p className="border-t pt-4 text-destructive">{row.retMsg}</p>
        )}
      </CardContent>
    </Card>
  );
}
