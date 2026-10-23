import "server-only";

import fs from "node:fs";
import path from "node:path";

/** Shared DATA_DIR resolution for SQLite reads (compose sets DATA_DIR=/data). */
export function resolveDataDir(): string {
  const fromEnv = process.env.DATA_DIR?.trim();
  if (fromEnv) {
    return path.isAbsolute(fromEnv) ? fromEnv : path.join(process.cwd(), fromEnv);
  }
  const candidates: string[] = [];
  let dir = process.cwd();
  for (let i = 0; i < 6; i++) {
    candidates.push(path.join(dir, "data"));
    const parent = path.dirname(dir);
    if (parent === dir) break;
    dir = parent;
  }
  for (const candidate of candidates) {
    const normalized = path.normalize(candidate);
    if (
      fs.existsSync(path.join(normalized, "calculation_signal.sqlite")) ||
      fs.existsSync(path.join(normalized, "execution.sqlite"))
    ) {
      return normalized;
    }
  }
  for (const candidate of candidates) {
    if (fs.existsSync(candidate)) {
      return path.normalize(candidate);
    }
  }
  return path.join(process.cwd(), "data");
}
