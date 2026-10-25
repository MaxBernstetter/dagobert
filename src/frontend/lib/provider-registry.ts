/** Plugged-in market-data providers (code registry). Always listed on Ingestion. */

export type ProviderConfigField =
  | {
      key: string;
      label: string;
      kind: "string";
      secret?: false;
      description?: string;
      placeholder?: string;
      default?: string;
    }
  | {
      key: string;
      label: string;
      kind: "string";
      secret: true;
      description?: string;
      placeholder?: string;
    };

export type PluggedProvider = {
  id: string;
  label: string;
  description?: string;
  fields: ProviderConfigField[];
};

export const PLUGGED_PROVIDERS: PluggedProvider[] = [
  {
    id: "binance",
    label: "Binance",
    description: "Public spot klines (no API key).",
    fields: [
      {
        key: "baseUrl",
        label: "Base URL",
        kind: "string",
        description: "Public data API host for klines and exchangeInfo.",
        placeholder: "https://data-api.binance.vision",
        default: "https://data-api.binance.vision",
      },
    ],
  },
];

export function pluggedProvider(id: string): PluggedProvider | undefined {
  return PLUGGED_PROVIDERS.find((p) => p.id === id);
}

export function isPluggedProviderId(id: string): boolean {
  return PLUGGED_PROVIDERS.some((p) => p.id === id);
}
