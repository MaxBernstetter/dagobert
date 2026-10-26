import { apiUrl } from "@/lib/api";

export type ReadyProvider = { id: string; label: string };

// The orchestrator has no "ready" route. The provider list carries a status per provider,
// and a provider is usable for experiments once its status is "ready".
export async function fetchReadyProviders(): Promise<ReadyProvider[]> {
  const res = await fetch(apiUrl("/providers"), { cache: "no-store" });
  if (!res.ok) {
    throw new Error((await res.text()) || `HTTP ${res.status}`);
  }
  const body = (await res.json()) as { providers?: (ReadyProvider & { status: string })[] };
  return (body.providers ?? []).filter((p) => p.status === "ready");
}
