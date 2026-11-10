# Grooming: experiment-ready strategies

> Historical: this note describes the removed orchestrator and its REST routes (`/api/v1/...`). The current runtime is in [docs/architecture/local-stack.md](../architecture/local-stack.md); the APIs are Connect services in `proto/`.

## Problem

The Experiments UI needs:

1. Which strategies can run as multi-config instances (**readiness**).
2. Per-strategy **options** (label, description, value domain) so the create form can render inputs **dynamically** — different strategies expose different knobs.

Not every deployed playbook qualifies: the strategy must accept **runtime config** and the experiment job must validate and replay it.

Today:

Experiment Suite and both ready playbooks use **3h** aggregation (`AGGREGATION_DURATION_MS` unset).

| Playbook | Deployed | Reads `StrategyConfig` | Reports options | Experiment-ready |
|----------|----------|------------------------|-----------------|------------------|
| ma-cross | yes | yes (`fastHr` / `slowHr`) | yes (enum hours) | **yes** |
| signal-consensus | yes | yes (threshold knobs) | yes (number ranges) | **yes** |

## API contract (target)

`GET /api/v1/experiments/strategies` → `{ strategies: ExperimentReadyStrategy[] }`

```ts
type ExperimentReadyStrategy = {
  id: string;
  label: string;
  description?: string;
  experimentReady: true;
  options: ExperimentStrategyOption[]; // drives instance UI
};

// Discriminated by kind — UI switches widgets
type ExperimentStrategyOption =
  | { kind: "enum"; key; label; description?; valueType; values: { value; label }[]; default }
  | { kind: "number"; key; label; description?; min?; max?; step?; unit?; default };
```

Interim implementation: allowlist + option descriptors in `src/frontend/lib/experiment-strategies.ts`, which was removed with the previous Next app.

## Discussion: source of truth

Options to groom:

1. **Go registry export** — each ready strategy implements something like `ExperimentOptions() []Option` next to `ParseConfig` / `Validate`; Next proxies or embeds generated JSON at build.
2. **Shared manifest** (YAML/JSON) consumed by Go (validate) and Next (render) — still manual, no code drift between languages if both load it.
3. **Capability interface** — `ConfigurableStrategy` with `ParseConfig`, `DefaultConfig`, `Options()`; registry discovers implementers.

**Recommendation:** (3) in Go as source of truth for readiness **and** options; `GET /api/v1/experiments/strategies` stays the frontend contract. Adding a strategy = implement config + options once; UI and job both consume the same descriptors.

## Acceptance when groomed

- New experiment-ready strategy: one Go change; dropdown + instance inputs update without a TS allowlist edit.
- Options include enough metadata to render without strategy-specific UI branches (beyond `kind`).
- System test: every listed strategy validates a sample config built from option defaults.
