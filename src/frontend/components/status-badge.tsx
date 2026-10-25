import { Badge } from "@/components/ui/badge";
import type { ActivityRow } from "@/types/read";

function statusLabel(kind: ActivityRow["kind"]): string {
  switch (kind) {
    case "fill":
      return "filled";
    case "rejection":
      return "rejected";
    case "no-trade":
      return "no-trade";
    case "pending":
      return "pending";
    case "not-yet-filled":
      return "not-yet-filled";
    default:
      return "—";
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
    case "pending":
      return "outline";
    default:
      return "outline";
  }
}

export function StatusBadge({ kind }: { kind: ActivityRow["kind"] }) {
  return <Badge variant={statusVariant(kind)}>{statusLabel(kind)}</Badge>;
}

/** Exchange order status when execution has touched the venue; otherwise em dash. */
export function rowVenueStatus(row: ActivityRow): string {
  if (row.kind === "fill" || row.kind === "pending") {
    return row.orderStatus ?? (row.kind === "fill" ? "Filled" : "Pending");
  }
  if (row.kind === "rejection" && row.retMsg) {
    return row.retMsg;
  }
  return "—";
}

/** True when the venue line says something the execution badge does not already say. */
export function venueStatusAddsDetail(row: ActivityRow): boolean {
  const venue = rowVenueStatus(row);
  if (venue === "—") return false;
  if (row.kind === "fill" && venue.toLowerCase() === "filled") return false;
  if (row.kind === "pending" && venue.toLowerCase() === "pending") return false;
  if (row.kind === "rejection" && row.retMsg && venue === row.retMsg) return false;
  return true;
}
