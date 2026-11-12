import { createClient } from "@connectrpc/connect";
import { transport } from "@/lib/auth";

import type { ProviderPublicView, ProviderStatus } from "@/lib/provider-catalog-types";
import type { ProviderConfigField } from "@/lib/provider-registry";
import {
  ProviderService,
  type ProviderView,
} from "@/lib/gen/tradingcopilot/v1/provider_service_pb";

/**
 * The Connectors contract (ProviderService, Connect-RPC, binary protobuf), same origin as the Experiment API
 * unless VITE_EXPERIMENT_API_URL names another host. Secret values are write-only: a view lists only the names of
 * the secrets that are set.
 */
const client = createClient(ProviderService, transport);

/** The view the Connectors page renders, mapped from the generated message. */
function toPublicView(v: ProviderView): ProviderPublicView {
  return {
    id: v.id,
    label: v.label,
    description: v.description || undefined,
    fields: v.fields.map((f): ProviderConfigField =>
      f.kind === "secret"
        ? {
            key: f.key,
            label: f.label,
            kind: "string",
            secret: true,
            description: f.description || undefined,
            placeholder: f.placeholder || undefined,
          }
        : {
            key: f.key,
            label: f.label,
            kind: "string",
            description: f.description || undefined,
            placeholder: f.placeholder || undefined,
            default: f.default || undefined,
          },
    ),
    activated: v.activated,
    status: v.status as ProviderStatus,
    error: v.error,
    progress: v.progress
      ? { label: v.progress.label, done: v.progress.done ?? null, total: v.progress.total ?? null }
      : null,
    config: { ...v.config },
    secretKeysConfigured: [...v.secretKeysConfigured],
    updatedAtMs: Number(v.updatedAtMs),
  };
}

export async function listProviders(): Promise<ProviderPublicView[]> {
  const res = await client.listProviders({});
  return res.providers.map(toPublicView);
}

export async function saveProviderSettings(
  id: string,
  config: Record<string, string>,
  secrets: Record<string, string>,
): Promise<ProviderPublicView> {
  const res = await client.saveProviderSettings({ id, config, secrets });
  return toPublicView(res.provider!);
}

/** Runs the setup before it resolves: the view is ready or failed. */
export async function activateProvider(id: string): Promise<ProviderPublicView> {
  const res = await client.activateProvider({ id });
  return toPublicView(res.provider!);
}

export async function deactivateProvider(id: string): Promise<ProviderPublicView> {
  const res = await client.deactivateProvider({ id });
  return toPublicView(res.provider!);
}

/** Stored snapshot when ready, otherwise the live markets (nothing stored). */
export async function listProviderPairs(id: string): Promise<{ base: string; quote: string }[]> {
  const res = await client.listProviderPairs({ id });
  return res.pairs.map((p) => ({ base: p.base, quote: p.quote }));
}
