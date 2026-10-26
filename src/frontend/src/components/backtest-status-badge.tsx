import { Badge } from "@/components/ui/badge";
import {
  catalogStatusBadgeClass,
  catalogStatusBadgeVariant,
} from "@/lib/catalog-status";
import { cn } from "@/lib/utils";

/** Simple status chip for experiment instances (no stage progress). */
export function CatalogStatusBadge({ status }: { status: string }) {
  const label = status.charAt(0).toUpperCase() + status.slice(1);
  return (
    <Badge
      variant={catalogStatusBadgeVariant(status)}
      className={cn(catalogStatusBadgeClass(status))}
    >
      {label}
    </Badge>
  );
}
