import { DataBrowser } from "@/components/data-browser";
import { readAlgos } from "@/lib/read";

export const dynamic = "force-dynamic";

export default function DataPage() {
  const algos = readAlgos();

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Data</h1>
        <p className="text-sm text-muted-foreground">
          Pick one position. Candles are Binance public klines. Each algorithm version stays in the history.
        </p>
      </div>
      <DataBrowser algos={algos} />
    </div>
  );
}
