-- name: ListExperiments :many
SELECT *
FROM experiment
ORDER BY created_at_ms DESC;

-- name: GetExperiment :one
SELECT *
FROM experiment
WHERE id = $1;

-- name: InsertExperiment :one
INSERT INTO experiment (
  id, name, description, status, error,
  provider, base, quote, from_ms, to_ms,
  initial_usdc, initial_usdt, initial_base,
  target_weight_pct, cash_reserve_quote, min_order_quote,
  redownload_missing, algo, spread_bps, slippage_bps, fee_bps,
  parallelism,
  created_at_ms, started_at_ms, finished_at_ms,
  progress_stage, progress_stages, progress_stage_label,
  progress_instance_done, progress_instance_total
) VALUES (
  $1, $2, $3, $4, $5,
  $6, $7, $8, $9, $10,
  $11, $12, $13,
  $14, $15, $16,
  $17, $18, $19, $20, $21,
  $30,
  $22, $23, $24,
  $25, $26, $27,
  $28, $29
)
RETURNING *;

-- name: UpdateExperimentStatus :exec
UPDATE experiment
SET status = $2,
    error = $3,
    started_at_ms = COALESCE($4, started_at_ms),
    finished_at_ms = $5,
    progress_stage = COALESCE($6, progress_stage),
    progress_stages = COALESCE($7, progress_stages),
    progress_stage_label = COALESCE($8, progress_stage_label),
    progress_instance_done = COALESCE($9, progress_instance_done),
    progress_instance_total = COALESCE($10, progress_instance_total)
WHERE id = $1;

-- name: UpdateExperimentProgress :exec
UPDATE experiment
SET progress_stage = $2,
    progress_stages = $3,
    progress_stage_label = $4,
    progress_instance_done = $5,
    progress_instance_total = $6
WHERE id = $1
  AND status <> 'cancelled';

-- name: MarkExperimentDraft :exec
UPDATE experiment
SET status = 'draft',
    error = '',
    progress_stage_label = '',
    progress_instance_done = $2,
    progress_instance_total = $2
WHERE id = $1;

-- name: MarkExperimentFailed :exec
UPDATE experiment
SET status = 'failed',
    error = $2,
    finished_at_ms = $3,
    progress_stage_label = 'Failed'
WHERE id = $1
  AND status <> 'cancelled';

-- name: MarkExperimentCompleted :exec
UPDATE experiment
SET status = 'completed',
    error = '',
    finished_at_ms = $2,
    progress_stage = 4,
    progress_stages = 4,
    progress_stage_label = 'Instance simulation'
WHERE id = $1
  AND status <> 'cancelled';

-- name: MarkExperimentPartial :exec
UPDATE experiment
SET status = 'partial',
    error = $2,
    finished_at_ms = $3,
    progress_stage = 4,
    progress_stages = 4,
    progress_stage_label = 'Instance simulation'
WHERE id = $1
  AND status <> 'cancelled';

-- name: MarkExperimentRunningKeep :execrows
UPDATE experiment
SET status = 'running',
    started_at_ms = $2,
    finished_at_ms = NULL,
    error = '',
    progress_stage = 1,
    progress_stages = 4,
    progress_stage_label = 'Preparing',
    progress_instance_done = 0,
    progress_instance_total = $3
WHERE id = $1
  AND status <> 'cancelled';

-- name: MarkExperimentRetryFailed :execrows
UPDATE experiment
SET status = 'running',
    started_at_ms = $2,
    finished_at_ms = NULL,
    error = '',
    progress_stage = 1,
    progress_stages = 4,
    progress_stage_label = 'Preparing',
    progress_instance_done = 0,
    progress_instance_total = $3
WHERE id = $1
  AND status <> 'cancelled';

-- name: MarkExperimentRunning :exec
UPDATE experiment
SET status = 'running',
    started_at_ms = $2,
    finished_at_ms = NULL,
    error = '',
    progress_stage = 1,
    progress_stages = 4,
    progress_stage_label = 'Starting',
    progress_instance_done = 0
WHERE id = $1;

-- name: MarkExperimentRunningUnlessCancelled :execrows
UPDATE experiment
SET status = 'running',
    started_at_ms = $2,
    finished_at_ms = NULL,
    error = '',
    progress_stage = 1,
    progress_stages = 4,
    progress_stage_label = 'Starting',
    progress_instance_done = 0
WHERE id = $1
  AND status <> 'cancelled';

-- name: MarkExperimentCancelled :exec
UPDATE experiment
SET status = 'cancelled',
    finished_at_ms = $2,
    error = 'cancelled',
    progress_stage_label = 'Cancelled'
WHERE id = $1;

-- name: DeleteExperiment :exec
DELETE FROM experiment WHERE id = $1;

-- name: CountFailedInstances :one
SELECT COUNT(*)::bigint AS n
FROM experiment_instance
WHERE experiment_id = $1 AND status = 'failed';

-- name: ListDistinctPlaybooks :many
SELECT DISTINCT playbook
FROM experiment_instance
WHERE experiment_id = $1 AND playbook <> ''
ORDER BY playbook;

-- name: ListDistinctSymbols :many
SELECT DISTINCT symbol
FROM experiment_instance
WHERE experiment_id = $1 AND symbol <> ''
ORDER BY symbol;
