
import { apiUrl } from "@/lib/api";
import { useEffect, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

type TradingSecretField = {
  key: string;
  label: string;
  configured: boolean;
  masked: string;
};

type TradingConnector = {
  id: string;
  label: string;
  description: string;
  host: string;
  configured: boolean;
  secrets: TradingSecretField[];
};

export function TradingConnectorsPanel({ className }: { className?: string }) {
  const [connectors, setConnectors] = useState<TradingConnector[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const res = await fetch(apiUrl("/connectors/trading"), { cache: "no-store" });
        if (!res.ok) {
          throw new Error((await res.text()) || `HTTP ${res.status}`);
        }
        const body = (await res.json()) as { connectors?: TradingConnector[] };
        if (!cancelled) {
          setConnectors(body.connectors ?? []);
          setError("");
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : String(err));
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  if (loading) {
    return (
      <p className={cn("text-sm text-muted-foreground", className)}>
        Loading trading connectors…
      </p>
    );
  }

  return (
    <div className={cn("grid gap-3", className)}>
      {error ? (
        <p className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </p>
      ) : null}

      {connectors.length === 0 ? (
        <p className="text-sm text-muted-foreground">No trading connectors.</p>
      ) : (
        connectors.map((c) => (
          <div key={c.id} className="rounded-lg border p-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <h3 className="font-semibold">{c.label}</h3>
                <p className="mt-0.5 text-sm text-muted-foreground">{c.description}</p>
              </div>
              <Badge variant={c.configured ? "outline" : "secondary"}>
                {c.configured ? "Configured" : "Not configured"}
              </Badge>
            </div>

            <dl className="mt-4 grid gap-3 text-sm sm:grid-cols-2">
              <div className="grid gap-0.5">
                <dt className="text-muted-foreground">Host</dt>
                <dd className="font-mono text-xs break-all">{c.host}</dd>
              </div>
              {c.secrets.map((s) => (
                <div key={s.key} className="grid gap-0.5">
                  <dt className="text-muted-foreground">{s.label}</dt>
                  <dd className="font-mono text-xs">
                    {s.configured ? s.masked : "—"}
                  </dd>
                </div>
              ))}
            </dl>
          </div>
        ))
      )}
    </div>
  );
}
