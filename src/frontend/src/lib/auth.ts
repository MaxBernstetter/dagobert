import type { Interceptor } from "@connectrpc/connect";
import { Code, ConnectError } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

/**
 * Sign-in through the API's own endpoints (POST /auth/login, /auth/refresh; src/internal/auth), which talk to Amazon
 * Cognito. The access token goes to every Connect call as a bearer token. Tokens live in localStorage so a reload keeps
 * the session. Same origin unless VITE_EXPERIMENT_API_URL names another host.
 */
const base = (import.meta.env.VITE_EXPERIMENT_API_URL ?? "").replace(/\/$/, "");
const STORAGE_KEY = "dagobert.auth";
const EXPIRY_SKEW_MS = 30_000;

type Session = { accessToken: string; refreshToken: string; expiresAt: number };

const listeners = new Set<() => void>();
let session: Session | null = load();

function load(): Session | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? (JSON.parse(raw) as Session) : null;
  } catch {
    return null;
  }
}

function save(next: Session | null): void {
  session = next;
  if (next) localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
  else localStorage.removeItem(STORAGE_KEY);
  listeners.forEach((l) => l());
}

type Tokens = { accessToken: string; refreshToken?: string; expiresIn: number };

async function post(path: string, body: unknown): Promise<Tokens> {
  const res = await fetch(`${base}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  const json = (await res.json().catch(() => ({}))) as Partial<Tokens> & { message?: string };
  if (!res.ok) throw new Error(json.message ?? `Sign-in failed (${res.status})`);
  return json as Tokens;
}

export async function login(username: string, password: string): Promise<void> {
  const r = await post("/auth/login", { username, password });
  save({ accessToken: r.accessToken, refreshToken: r.refreshToken ?? "", expiresAt: Date.now() + r.expiresIn * 1000 });
}

export function logout(): void {
  save(null);
}

export function isSignedIn(): boolean {
  return session !== null;
}

export function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

let refreshing: Promise<void> | null = null;

async function accessToken(): Promise<string | null> {
  if (!session) return null;
  if (session.expiresAt - EXPIRY_SKEW_MS > Date.now()) return session.accessToken;
  const refreshToken = session.refreshToken;
  refreshing ??= post("/auth/refresh", { refreshToken })
    .then((r) => save({ accessToken: r.accessToken, refreshToken, expiresAt: Date.now() + r.expiresIn * 1000 }))
    .catch(() => save(null))
    .finally(() => {
      refreshing = null;
    });
  await refreshing;
  return session?.accessToken ?? null;
}

const bearer: Interceptor = (next) => async (req) => {
  const token = await accessToken();
  if (token) req.header.set("Authorization", `Bearer ${token}`);
  try {
    return await next(req);
  } catch (err) {
    if (ConnectError.from(err).code === Code.Unauthenticated) logout();
    throw err;
  }
};

/** The transport of every Connect service: same origin unless VITE_EXPERIMENT_API_URL names another host. */
export const transport = createConnectTransport({
  baseUrl: base,
  useBinaryFormat: true,
  interceptors: [bearer],
});
