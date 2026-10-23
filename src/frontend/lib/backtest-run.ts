import "server-only";

import { spawn } from "node:child_process";
import path from "node:path";

import { resolveDataDir } from "@/lib/data-dir";

const orchestratorBase = () => process.env.ORCHESTRATOR_URL ?? "http://orchestrator:8090";

export type BacktestRunTriggerResult =
  | { ok: true; status: number; body: string; contentType: string | null }
  | { ok: false; status: number; error: string };

/** Start a catalogued backtest via the orchestrator, or docker when configured. */
export async function triggerBacktestRun(id: string): Promise<BacktestRunTriggerResult> {
  const viaOrchestrator = await postOrchestrator(id);
  if (viaOrchestrator.status !== 404) {
    if (!viaOrchestrator.ok) {
      return {
        ok: false,
        status: viaOrchestrator.status,
        error: viaOrchestrator.body.trim() || `orchestrator returned ${viaOrchestrator.status}`,
      };
    }
    return {
      ok: true,
      status: viaOrchestrator.status,
      body: viaOrchestrator.body,
      contentType: viaOrchestrator.contentType,
    };
  }

  if (process.env.BACKTEST_DOCKER_FALLBACK === "1") {
    const docker = spawnBacktestDocker(id);
    if (!docker.ok) {
      return { ok: false, status: 502, error: docker.error };
    }
    return {
      ok: true,
      status: 202,
      body: JSON.stringify({ id, running: true, via: "docker" }),
      contentType: "application/json",
    };
  }

  return {
    ok: false,
    status: 502,
    error:
      "Orchestrator does not expose POST /backtest/run (stale image). Rebuild and restart: docker compose build orchestrator && docker compose up -d orchestrator. " +
      "For local Next.js without orchestrator, set BACKTEST_DOCKER_FALLBACK=1 and ensure the trading-copilot-backtest image exists.",
  };
}

async function postOrchestrator(id: string): Promise<{
  ok: boolean;
  status: number;
  body: string;
  contentType: string | null;
}> {
  const res = await fetch(`${orchestratorBase()}/backtest/run`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ id }),
  });
  const text = await res.text();
  return {
    ok: res.ok,
    status: res.status,
    body: text,
    contentType: res.headers.get("Content-Type"),
  };
}

function hostDataDirForDocker(): string {
  const fromEnv = process.env.HOST_DATA_DIR?.trim();
  if (fromEnv) {
    return path.isAbsolute(fromEnv) ? fromEnv : path.join(process.cwd(), fromEnv);
  }
  return resolveDataDir();
}

function spawnBacktestDocker(id: string): { ok: true } | { ok: false; error: string } {
  const dataDir = hostDataDirForDocker();
  const args = [
    "run",
    "--rm",
    "-e",
    "DATA_DIR=/data",
    "-e",
    `BACKTEST_ID=${id}`,
    "-e",
    "VENUE_SIMULATE=1",
    "-e",
    `BINANCE_BASE_URL=${process.env.BINANCE_BASE_URL ?? "https://data-api.binance.vision"}`,
    "-v",
    `${dataDir}:/data`,
    "trading-copilot-backtest",
  ];
  try {
    const child = spawn("docker", args, { stdio: "ignore", detached: true });
    child.unref();
  } catch (err) {
    const message = err instanceof Error ? err.message : "docker spawn failed";
    return { ok: false, error: message };
  }
  return { ok: true };
}
