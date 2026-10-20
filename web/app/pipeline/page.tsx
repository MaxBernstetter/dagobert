import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { readPipeline } from "@/lib/read";

export const dynamic = "force-dynamic";

function formatCandleClose(ms: number | null): string {
  if (ms == null || !ms) return "—";
  return new Date(ms).toISOString().replace("T", " ").replace(/\.\d{3}Z$/, " UTC");
}

export default function PipelinePage() {
  const rows = readPipeline();

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Pipeline</h1>
        <p className="text-sm text-muted-foreground">
          Service status for a candle close (from local pipeline history file).
        </p>
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Service</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Start</TableHead>
            <TableHead>Finish</TableHead>
            <TableHead>Retries</TableHead>
            <TableHead>Error</TableHead>
            <TableHead>Candle close</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={row.service}>
              <TableCell className="font-medium">{row.service}</TableCell>
              <TableCell>
                <Badge
                  variant={
                    row.status === "unavailable" ? "outline" : "secondary"
                  }
                >
                  {row.status}
                </Badge>
              </TableCell>
              <TableCell className="text-xs whitespace-nowrap">
                {row.start ?? "—"}
              </TableCell>
              <TableCell className="text-xs whitespace-nowrap">
                {row.finish ?? "—"}
              </TableCell>
              <TableCell>{row.retries ?? "—"}</TableCell>
              <TableCell className="max-w-xs truncate text-xs">
                {row.error ?? "—"}
              </TableCell>
              <TableCell className="text-xs whitespace-nowrap">
                {formatCandleClose(row.candleClose)}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
