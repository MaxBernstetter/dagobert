import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import type { ActivityRow } from "@/lib/read";
import { StatusBadge, rowStatusLine } from "@/components/status-badge";

function formatCandleClose(ms: number): string {
  if (!ms) return "—";
  return new Date(ms).toISOString().replace("T", " ").replace(/\.\d{3}Z$/, " UTC");
}

export function CandleDetail({ row }: { row: ActivityRow | null }) {
  if (!row) {
    return (
      <Card className="h-full">
        <CardHeader>
          <CardTitle>Detail</CardTitle>
          <CardDescription>Select a candle from the activity list.</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  return (
    <Card className="h-full">
      <CardHeader>
        <div className="flex flex-wrap items-center gap-2">
          <CardTitle className="text-lg">{row.symbol}</CardTitle>
          <StatusBadge kind={row.kind} />
        </div>
        <CardDescription>{formatCandleClose(row.candleClose)}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 text-sm">
        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2">
          <dt className="text-muted-foreground">Playbook</dt>
          <dd>{row.playbook || "—"}</dd>
          <dt className="text-muted-foreground">Rule</dt>
          <dd className="font-mono text-xs break-all">{row.rule ?? "—"}</dd>
          <dt className="text-muted-foreground">Decision</dt>
          <dd>{row.decision ?? "—"}</dd>
          <dt className="text-muted-foreground">Size</dt>
          <dd>{row.size != null ? String(row.size) : "—"}</dd>
          <dt className="text-muted-foreground">Status</dt>
          <dd>{rowStatusLine(row)}</dd>
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
