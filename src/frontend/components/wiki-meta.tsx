import { Badge } from "@/components/ui/badge";
import type { WikiMeta } from "@/lib/wiki";

export function WikiMetaTable({ meta }: { meta: WikiMeta }) {
  const rows: { label: string; value: string }[] = [
    { label: "Slug", value: meta.slug },
    { label: "Kind", value: meta.kind },
  ];
  if (meta.status) rows.push({ label: "Status", value: meta.status });
  if (meta.unit) rows.push({ label: "Unit", value: meta.unit });
  if (meta.algorithm) rows.push({ label: "Algorithm", value: meta.algorithm });
  if (meta.playbook_file) rows.push({ label: "Playbook file", value: meta.playbook_file });
  if (meta.venue) rows.push({ label: "Venue", value: meta.venue });
  if (meta.price_source) rows.push({ label: "Price source", value: meta.price_source });
  if (meta.definition) {
    rows.push({ label: "Definition", value: JSON.stringify(meta.definition) });
  }
  if (meta.symbols?.length) {
    rows.push({ label: "Symbols", value: meta.symbols.join(", ") });
  }
  if (meta.inputs?.length) {
    rows.push({ label: "Inputs", value: meta.inputs.join("; ") });
  }

  return (
    <div className="rounded-lg border bg-card p-4 text-sm">
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <h2 className="font-semibold">Metadata</h2>
        {meta.status ? <Badge variant="secondary">{meta.status}</Badge> : null}
        {meta.unit ? <Badge variant="outline">{meta.unit}</Badge> : null}
      </div>
      <dl className="grid gap-2 sm:grid-cols-[minmax(7rem,auto)_1fr]">
        {rows.map((row) => (
          <div key={row.label} className="contents">
            <dt className="text-muted-foreground">{row.label}</dt>
            <dd className="font-mono text-xs break-all sm:text-sm">{row.value}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}
