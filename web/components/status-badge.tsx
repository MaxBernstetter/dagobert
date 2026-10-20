import { Badge } from "@/components/ui/badge";
import type { ActivityRow } from "@/lib/read";

function statusLabel(kind: ActivityRow["kind"]): string {
  switch (kind) {
    case "fill":
      return "Filled";
    case "rejection":
      return "rejected";
    case "no-trade":
      return "no-trade";
    case "pending":
    case "not-yet-filled":
    default:
      return "not-yet-filled";
  }
}

function statusVariant(
  kind: ActivityRow["kind"]
): "default" | "destructive" | "secondary" | "outline" {
  switch (kind) {
    case "fill":
      return "default";
    case "rejection":
      return "destructive";
    case "no-trade":
      return "secondary";
    default:
      return "outline";
  }
}

export function StatusBadge({ kind }: { kind: ActivityRow["kind"] }) {
  return <Badge variant={statusVariant(kind)}>{statusLabel(kind)}</Badge>;
}

export function rowStatusLine(row: ActivityRow): string {
  if (row.kind === "rejection" && row.retMsg) {
    return row.retMsg;
  }
  if (row.kind === "fill") {
    return row.orderStatus ?? "Filled";
  }
  if (row.kind === "no-trade") {
    return row.decision ?? "no-trade";
  }
  if (row.kind === "pending") {
    return row.orderStatus ?? "Pending";
  }
  return row.decision ?? "—";
}
