/** Current frontend HTTP API prefix. Bump when introducing a breaking surface. */
export const API_VERSION = "v1" as const;

export const API_BASE = `/api/${API_VERSION}` as const;

/** Build a versioned API path. `path` may be `/experiments` or `experiments`. */
export function apiUrl(path: string): string {
  const normalized = path.startsWith("/") ? path : `/${path}`;
  return `${API_BASE}${normalized}`;
}
