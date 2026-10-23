import type { IndicatorSeries } from "@/lib/wiki";

export function WikiSeriesTable({ series }: { series: IndicatorSeries[] }) {
  if (series.length === 0) {
    return null;
  }
  return (
    <div className="rounded-lg border bg-card p-4 text-sm">
      <h2 className="mb-3 font-semibold">Deployed series (v1)</h2>
      <p className="mb-3 text-xs text-muted-foreground">
        Keys such as <code className="rounded bg-muted px-1">sma-6h</code> identify a calculation for operators and the
        Wiki. KPI rows store the JSON definition only.
      </p>
      <table className="w-full border-collapse text-left text-xs">
        <thead>
          <tr className="border-b">
            <th className="py-2 pr-3 font-medium">Key</th>
            <th className="py-2 font-medium">KPI definition</th>
          </tr>
        </thead>
        <tbody>
          {series.map((row) => (
            <tr key={row.key} className="border-b border-border/60">
              <td className="py-2 pr-3 font-mono">{row.key}</td>
              <td className="py-2 font-mono break-all">{JSON.stringify(row.definition)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
