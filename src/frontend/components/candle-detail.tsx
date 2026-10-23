import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import type { ActivityRow, LiveState } from "@/lib/read";
import {
  executionKindHelp,
  rowExecutionHint,
} from "@/lib/execution-guidance";
import { formatPlaybookTitle } from "@/lib/strategy-groups";
import { STRATEGY_COLUMN_HELP } from "@/lib/strategy-help";
import { HelpTip } from "@/components/help-tip";
import { StatusBadge, rowVenueStatus } from "@/components/status-badge";

function formatCandleClose(ms: number): string {
  if (!ms) return "—";
  return new Date(ms).toISOString().replace("T", " ").replace(/\.\d{3}Z$/, " UTC");
}

export function CandleDetail({
  row,
  live,
}: {
  row: ActivityRow | null;
  live: LiveState;
}) {
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
