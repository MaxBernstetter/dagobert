import { DataBrowser } from "@/components/data-browser";
import { DataLakeSummary } from "@/components/data-lake-summary";
import { readAlgos, readDataLake } from "@/lib/read";

export const dynamic = "force-dynamic";

export default function DataPage() {
  const algos = readAlgos();
  const lake = readDataLake();

  return (
    <div className="min-h-0 flex-1 space-y-8 overflow-y-auto">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Data</h1>
        <p className="text-sm text-muted-foreground">
          Ingested candles and technical-indicator KPIs. Pick a position to inspect bars and indicator series.
          Market decisions and orders live on{" "}
          <a href="/strategy" className="underline underline-offset-2">Strategy</a>.
        </p>
      </div>
      <DataLakeSummary lake={lake} />
      <DataBrowser algos={algos} />
    </div>
  );
}
