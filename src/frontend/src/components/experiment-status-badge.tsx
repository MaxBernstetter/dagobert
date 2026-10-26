import { Badge } from "@/components/ui/badge";
import {
  catalogStatusBadgeClass,
  catalogStatusBadgeVariant,
} from "@/lib/catalog-status";
import type { ExperimentRecord } from "@/lib/experiment-shared";
import { experimentStatusLabel } from "@/lib/experiment-shared";
import { cn } from "@/lib/utils";

export function ExperimentStatusBadge({ record }: { record: ExperimentRecord }) {
  return (
    <Badge
      variant={catalogStatusBadgeVariant(record.status)}
      className={cn(catalogStatusBadgeClass(record.status))}
    >
      {experimentStatusLabel(record)}
    </Badge>
  );
}
