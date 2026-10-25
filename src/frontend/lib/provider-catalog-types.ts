import type { ProviderConfigField } from "@/lib/provider-registry";

export type ProviderStatus = "inactive" | "setting_up" | "ready" | "failed";

export type ProviderProgress = {
  label: string;
  done: number | null;
  total: number | null;
};

export type ProviderCatalogEntry = {
  activated: boolean;
  status: ProviderStatus;
  error: string;
  progress: ProviderProgress | null;
  config: Record<string, string>;
  secretKeysConfigured: string[];
  updatedAtMs: number;
};

export type ProviderPublicView = {
  id: string;
  label: string;
  description?: string;
  fields: ProviderConfigField[];
} & ProviderCatalogEntry;
