import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { DataLakeStats } from "@/lib/read";

function formatMs(ms: number | null): string {
  if (!ms) return "—";
  return new Date(ms).toISOString().replace("T", " ").replace(/\.\d{3}Z$/, " UTC");
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KiB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MiB`;
}

export function DataLakeSummary({ lake }: { lake: DataLakeStats }) {
  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Ingested hours</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-2xl font-semibold tabular-nums">{lake.hours}</p>
          <p className="text-xs text-muted-foreground">
            {lake.symbols} symbol{lake.symbols === 1 ? "" : "s"} · newest {formatMs(lake.newestHour)}
          </p>
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Technical indicators</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-2xl font-semibold tabular-nums">{lake.kpiRows}</p>
          <p className="text-xs text-muted-foreground">newest bar {formatMs(lake.newestBar)}</p>
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Parquet lake</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-2xl font-semibold tabular-nums">{formatBytes(lake.parquetBytes)}</p>
          <p className="text-xs text-muted-foreground">
            {lake.parquetUpdated ? `updated ${lake.parquetUpdated}` : "no files"}
          </p>
        </CardContent>
      </Card>
    </div>
  );
}
