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

-- name: DeleteFillsForInstance :exec
DELETE FROM experiment_instance_fill
WHERE experiment_id = $1 AND instance_id = $2;

-- name: InsertFill :exec
INSERT INTO experiment_instance_fill (
  id, experiment_id, instance_id, action_id, symbol, ts_ms, side, qty, price, fee, fee_asset
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
);

-- name: DeleteActionsForInstance :exec
DELETE FROM experiment_instance_action
WHERE experiment_id = $1 AND instance_id = $2;

-- name: DeleteTradeOutcomesForInstance :exec
DELETE FROM experiment_trade_outcome
WHERE instance_id = $1;

-- name: InsertTradeOutcome :exec
INSERT INTO experiment_trade_outcome (
  instance_id, ordinal, side, quote, percent
) VALUES (
  $1, $2, $3, $4, $5
);

-- name: ListTradeOutcomesForInstance :many
SELECT instance_id, ordinal, side, quote, percent
FROM experiment_trade_outcome
WHERE instance_id = $1
ORDER BY ordinal ASC;

-- name: ListTradeOutcomesForExperiment :many
SELECT o.instance_id, o.ordinal, o.side, o.quote, o.percent
FROM experiment_trade_outcome o
JOIN experiment_instance i ON i.id = o.instance_id
WHERE i.experiment_id = $1 AND i.status = 'completed'
ORDER BY o.instance_id, o.ordinal;

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

-- name: InsertAction :exec
INSERT INTO experiment_instance_action (
  id, experiment_id, instance_id, symbol, bar_open_ms, side, weight_pct, reason
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8
);
