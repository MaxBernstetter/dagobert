
import { apiUrl } from "@/lib/api";
import Link from "@/components/link";
import { useCallback, useEffect, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  catalogStatusBadgeClass,
  catalogStatusBadgeVariant,
} from "@/lib/catalog-status";
import type { ProviderPublicView } from "@/lib/provider-catalog-types";
import { invalidateProviderPairs } from "@/lib/provider-pairs";
import { cn } from "@/lib/utils";

function statusLabel(status: string): string {
  switch (status) {
    case "setting_up":
      return "Running";
    case "ready":
      return "Ready";
    case "failed":
      return "Failed";
    case "inactive":
      return "Inactive";
    default:
      return status;
  }
}

function ProgressBlock({ provider }: { provider: ProviderPublicView }) {
  const progress = provider.progress;
  if (provider.status !== "setting_up" && !progress) return null;

  const hasTotal =
    progress?.total != null &&
    Number.isFinite(progress.total) &&
    progress.total > 0;
  const done = progress?.done ?? 0;
  const total = progress?.total ?? 0;
  const pct = hasTotal ? Math.min(100, Math.round((done / total) * 100)) : null;

  return (
    <div className="grid gap-1.5">
      <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
        <span>{progress?.label || "Running setup…"}</span>
        {hasTotal ? (
          <span className="tabular-nums">
            {done}/{total}
          </span>
        ) : provider.status === "setting_up" ? (
          <span>…</span>
        ) : null}
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-muted">
        <div
          className={cn(
            "h-full rounded-full bg-sky-600 transition-all",
            pct == null && provider.status === "setting_up"
              ? "w-1/3 animate-pulse"
              : "",
          )}
          style={pct != null ? { width: `${pct}%` } : undefined}
        />
      </div>
    </div>
  );
}

export function IngestionProvidersPanel({ className }: { className?: string }) {
  const [providers, setProviders] = useState<ProviderPublicView[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [configDraft, setConfigDraft] = useState<Record<string, string>>({});
  const [secretsDraft, setSecretsDraft] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    const res = await fetch(apiUrl("/providers"), { cache: "no-store" });
    if (!res.ok) {
      throw new Error((await res.text()) || `HTTP ${res.status}`);
    }
    const body = (await res.json()) as { providers: ProviderPublicView[] };
    setProviders(body.providers ?? []);
    return body.providers ?? [];
  }, []);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const list = await refresh();
        if (!cancelled && list.length > 0) {
          setSelectedId((prev) => prev ?? list[0].id);
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
  }, [refresh]);

  const selected = providers.find((p) => p.id === selectedId) ?? null;

  useEffect(() => {
    if (!selected) return;
    const next: Record<string, string> = {};
    for (const field of selected.fields) {
      if (field.secret) continue;
      next[field.key] = selected.config[field.key] ?? field.default ?? "";
    }
    setConfigDraft(next);
    setSecretsDraft({});
  }, [selected?.id, selected?.updatedAtMs]);

  const anySettingUp = providers.some((p) => p.status === "setting_up");

  useEffect(() => {
    if (!anySettingUp) return;
    const timer = setInterval(() => {
      void refresh().catch(() => {
        /* keep last good state while polling */
      });
    }, 1000);
    return () => clearInterval(timer);
  }, [anySettingUp, refresh]);

  async function saveConfig() {
    if (!selected) return;
    setBusy(true);
    setError("");
    try {
      const secrets: Record<string, string> = {};
      for (const field of selected.fields) {
        if (!field.secret) continue;
        const value = secretsDraft[field.key];
        if (value != null && value !== "") secrets[field.key] = value;
      }
      const res = await fetch(apiUrl(`/providers/${encodeURIComponent(selected.id)}`), {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ config: configDraft, secrets }),
      });
      if (!res.ok) {
        throw new Error((await res.text()) || `HTTP ${res.status}`);
      }
      await refresh();
      setSecretsDraft({});
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  async function activate() {
    if (!selected) return;
    setBusy(true);
    setError("");
    try {
      const secrets: Record<string, string> = {};
      for (const field of selected.fields) {
        if (!field.secret) continue;
        const value = secretsDraft[field.key];
        if (value != null && value !== "") secrets[field.key] = value;
      }
      const res = await fetch(
        apiUrl(`/providers/${encodeURIComponent(selected.id)}/activate`),
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ config: configDraft, secrets }),
        },
      );
      if (!res.ok) {
        throw new Error((await res.text()) || `HTTP ${res.status}`);
      }
      await refresh();
      invalidateProviderPairs(selected.id);
      setSecretsDraft({});
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  async function deactivate() {
    if (!selected) return;
    setBusy(true);
    setError("");
    try {
      const res = await fetch(
        apiUrl(`/providers/${encodeURIComponent(selected.id)}/deactivate`),
        { method: "POST" },
      );
      if (!res.ok) {
        throw new Error((await res.text()) || `HTTP ${res.status}`);
      }
      await refresh();
      invalidateProviderPairs(selected.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  if (loading) {
    return (
      <p className={cn("text-sm text-muted-foreground", className)}>Loading providers…</p>
    );
  }

  return (
    <div className={cn("flex min-h-0 flex-col gap-4", className)}>
      <p className="text-sm text-muted-foreground">
        Activate market-data providers. Setup loads the asset catalog; only ready
        providers appear in{" "}
        <Link href="/experiments" className="underline underline-offset-2">
          Experiment Suite
        </Link>
        .
      </p>

      {error ? (
        <p className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </p>
      ) : null}

      <div className="grid min-h-0 flex-1 gap-4 lg:grid-cols-[minmax(14rem,18rem)_1fr]">
        <ul className="flex flex-col gap-1 overflow-auto rounded-lg border p-1">
          {providers.map((p) => (
            <li key={p.id}>
              <button
                type="button"
                onClick={() => setSelectedId(p.id)}
                className={cn(
                  "flex w-full items-center justify-between gap-2 rounded-md px-2.5 py-2 text-left text-sm transition-colors",
                  selectedId === p.id ? "bg-muted" : "hover:bg-muted/60",
                )}
              >
                <span className="font-medium">{p.label}</span>
                <BadgeStatus status={p.status} />
              </button>
            </li>
          ))}
        </ul>

        {selected ? (
          <div className="flex min-h-0 flex-col gap-4 overflow-auto rounded-lg border p-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <h2 className="font-semibold">{selected.label}</h2>
                {selected.description ? (
                  <p className="mt-0.5 text-sm text-muted-foreground">
                    {selected.description}
                  </p>
                ) : null}
              </div>
              <BadgeStatus status={selected.status} />
            </div>

            <ProgressBlock provider={selected} />

            {selected.status === "failed" && selected.error ? (
              <p className="text-sm text-destructive">{selected.error}</p>
            ) : null}

            {selected.status === "ready" && selected.progress?.done != null ? (
              <p className="text-sm text-muted-foreground">
                {selected.progress.done.toLocaleString()} pairs in catalog
              </p>
            ) : null}

            <div className="grid gap-3">
              {selected.fields.map((field) => {
                const isSecret = Boolean(field.secret);
                const configured =
                  isSecret && selected.secretKeysConfigured.includes(field.key);
                return (
                  <label key={field.key} className="grid gap-1">
                    <span className="text-sm text-muted-foreground">
                      {field.label}
                      {configured ? " (saved)" : ""}
                    </span>
                    <input
                      type={isSecret ? "password" : "text"}
                      className="w-full rounded-md border border-border bg-background px-2 py-1.5 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
                      placeholder={
                        isSecret && configured
                          ? "••••••••"
                          : field.placeholder
                      }
                      value={
                        isSecret
                          ? (secretsDraft[field.key] ?? "")
                          : (configDraft[field.key] ?? "")
                      }
                      disabled={busy || selected.status === "setting_up"}
                      onChange={(e) => {
                        if (isSecret) {
                          setSecretsDraft((prev) => ({
                            ...prev,
                            [field.key]: e.target.value,
                          }));
                        } else {
                          setConfigDraft((prev) => ({
                            ...prev,
                            [field.key]: e.target.value,
                          }));
                        }
                      }}
                      autoComplete="off"
                    />
                    {field.description ? (
                      <span className="text-xs text-muted-foreground">
                        {field.description}
                      </span>
                    ) : null}
                  </label>
                );
              })}
            </div>

            <div className="flex flex-wrap gap-2">
              <Button
                type="button"
                size="sm"
                disabled={busy || selected.status === "setting_up"}
                onClick={() => void saveConfig()}
                variant="outline"
              >
                Save config
              </Button>
              {selected.status === "ready" ? (
                <>
                  <Button
                    type="button"
                    size="sm"
                    disabled={busy}
                    onClick={() => void activate()}
                    variant="outline"
                  >
                    Refresh assets
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    disabled={busy}
                    onClick={() => void deactivate()}
                    variant="secondary"
                  >
                    Deactivate
                  </Button>
                </>
              ) : (
                <Button
                  type="button"
                  size="sm"
                  disabled={busy || selected.status === "setting_up"}
                  onClick={() => void activate()}
                >
                  {selected.status === "failed" ? "Retry activate" : "Activate"}
                </Button>
              )}
            </div>
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">No providers plugged in.</p>
        )}
      </div>
    </div>
  );
}

function BadgeStatus({ status }: { status: string }) {
  return (
    <Badge
      variant={catalogStatusBadgeVariant(status)}
      className={cn(catalogStatusBadgeClass(status))}
    >
      {statusLabel(status)}
    </Badge>
  );
}
