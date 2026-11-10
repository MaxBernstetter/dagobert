import { listProviders } from "@/lib/provider-service";

export type ReadyProvider = { id: string; label: string };

// The service has no "ready" call. The provider list carries a status per provider,
// and a provider is usable for experiments once its status is "ready".
export async function fetchReadyProviders(): Promise<ReadyProvider[]> {
  const providers = await listProviders();
  return providers
    .filter((p) => p.status === "ready")
    .map((p) => ({ id: p.id, label: p.label }));
}
