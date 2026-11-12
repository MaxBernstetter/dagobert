import { useState, useSyncExternalStore, type FormEvent, type ReactNode } from "react";
import { isSignedIn, login, subscribe } from "@/lib/auth";

/** Shows the app only after sign-in; the API refuses every call without a token anyway. */
export function LoginGate({ children }: { children: ReactNode }) {
  const signedIn = useSyncExternalStore(subscribe, isSignedIn);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  if (signedIn) return <>{children}</>;

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await login(username, password);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Sign-in failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex h-dvh items-center justify-center bg-background text-foreground">
      <form onSubmit={submit} className="flex w-72 flex-col gap-3">
        <h1 className="text-lg font-semibold tracking-tight">Sign in</h1>
        <input
          className="rounded border border-border bg-transparent px-3 py-2 text-sm"
          placeholder="Username"
          autoComplete="username"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
        />
        <input
          className="rounded border border-border bg-transparent px-3 py-2 text-sm"
          type="password"
          placeholder="Password"
          autoComplete="current-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
        {error && <p role="alert" className="text-sm text-red-500">{error}</p>}
        <button
          type="submit"
          disabled={busy || !username || !password}
          className="rounded bg-primary px-3 py-2 text-sm text-primary-foreground disabled:opacity-50"
        >
          {busy ? "Signing in…" : "Sign in"}
        </button>
      </form>
    </div>
  );
}
