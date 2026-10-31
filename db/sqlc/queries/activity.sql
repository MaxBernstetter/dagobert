-- name: DeletePeriodStatsForInstance :exec
DELETE FROM experiment_instance_period_stat
WHERE instance_id = $1;

-- name: UpsertPeriodStat :exec
INSERT INTO experiment_instance_period_stat (
  instance_id, period_kind, period_key, return_pct, trade_count, win_count
) VALUES (
  $1, $2, $3, $4, $5, $6
)
ON CONFLICT (instance_id, period_kind, period_key) DO UPDATE SET
  return_pct = EXCLUDED.return_pct,
  trade_count = EXCLUDED.trade_count,
  win_count = EXCLUDED.win_count;

-- name: ListCompletedPeriodStats :many
SELECT p.instance_id, p.period_kind, p.period_key, p.win_count, p.trade_count
FROM experiment_instance_period_stat p
JOIN experiment_instance i ON i.id = p.instance_id
WHERE i.experiment_id = $1 AND i.status = 'completed'
ORDER BY p.instance_id, p.period_kind, p.period_key;

-- name: ListInstancePeriodStats :many
SELECT instance_id, period_kind, period_key, return_pct, trade_count, win_count
FROM experiment_instance_period_stat
WHERE instance_id = $1
ORDER BY period_kind, period_key;

-- name: DeleteExperimentPerformance :exec
DELETE FROM experiment_performance WHERE experiment_id = $1;

-- name: UpsertExperimentPerformance :exec
INSERT INTO experiment_performance (experiment_id, wins, losses, breakeven)
VALUES ($1, $2, $3, $4)
ON CONFLICT (experiment_id) DO UPDATE SET
  wins = EXCLUDED.wins,
  losses = EXCLUDED.losses,
  breakeven = EXCLUDED.breakeven;

-- name: DeleteExperimentPeriodStats :exec
DELETE FROM experiment_period_stat WHERE experiment_id = $1;

-- name: InsertExperimentPeriodStat :exec
INSERT INTO experiment_period_stat (experiment_id, period_kind, period_key, wins, trades)
VALUES ($1, $2, $3, $4, $5);

-- name: DeleteExperimentOutcomeStats :exec
DELETE FROM experiment_outcome_stat WHERE experiment_id = $1;

-- name: InsertExperimentOutcomeStat :exec
INSERT INTO experiment_outcome_stat (
  experiment_id, side, unit, average, p10, p25, p50, p75, p90, p95
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
);

-- name: GetExperimentPerformance :one
SELECT experiment_id, wins, losses, breakeven, win_rate
FROM experiment_performance
WHERE experiment_id = $1;

-- name: ListExperimentPeriodStats :many
SELECT experiment_id, period_kind, period_key, wins, trades
FROM experiment_period_stat
WHERE experiment_id = $1
ORDER BY period_kind, period_key;

-- name: ListExperimentOutcomeStats :many
SELECT experiment_id, side, unit, average, p10, p25, p50, p75, p90, p95
FROM experiment_outcome_stat
WHERE experiment_id = $1
ORDER BY side, unit;

-- name: DeletePeriodStatsForInstances :exec
DELETE FROM experiment_instance_period_stat
WHERE instance_id = ANY($1::uuid[]);
