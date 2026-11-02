-- name: InsertInstance :one
INSERT INTO experiment_instance (
  id, experiment_id, label, playbook, symbol,
  config_hash, playbook_key, workspace_rel_path,
  status, error,
  cfg_fast_hr, cfg_slow_hr,
  cfg_rsi_bull, cfg_rsi_bear, cfg_vol_ratio_bull, cfg_vol_ratio_bear,
  cfg_atr_stop_mult, cfg_min_bars_flat_after_sell, cfg_min_bars_long_before_sell,
  strategy_config_json
) VALUES (
  $1, $2, $3, $4, $5,
  $6, $7, $8,
  $9, $10,
  $11, $12,
  $13, $14, $15, $16,
  $17, $18, $19, $20
)
RETURNING *;

-- name: GetInstance :one
SELECT *
FROM experiment_instance
WHERE experiment_id = $1 AND id = $2;

-- name: CountInstancesFiltered :one
SELECT COUNT(*)::bigint AS n
FROM experiment_instance
WHERE experiment_id = $1
  AND ($2::text = '' OR status = $2)
  AND ($3::text = '' OR UPPER(symbol) = UPPER($3))
  AND (
    $4::text = ''
    OR label ILIKE '%' || $4 || '%'
    OR symbol ILIKE '%' || $4 || '%'
    OR playbook ILIKE '%' || $4 || '%'
    OR status ILIKE '%' || $4 || '%'
  );

-- name: ListInstancesPage :many
SELECT *
FROM experiment_instance
WHERE experiment_id = $1
  AND ($2::text = '' OR status = $2)
  AND ($3::text = '' OR UPPER(symbol) = UPPER($3))
  AND (
    $4::text = ''
    OR label ILIKE '%' || $4 || '%'
    OR symbol ILIKE '%' || $4 || '%'
    OR playbook ILIKE '%' || $4 || '%'
    OR status ILIKE '%' || $4 || '%'
  )
ORDER BY
  CASE WHEN $5::text = 'sharpe' AND $6::text = 'desc' THEN metric_sharpe END DESC NULLS LAST,
  CASE WHEN $5::text = 'sharpe' AND $6::text = 'asc' THEN metric_sharpe END ASC NULLS LAST,
  CASE WHEN $5::text = 'sortino' AND $6::text = 'desc' THEN metric_sortino END DESC NULLS LAST,
  CASE WHEN $5::text = 'sortino' AND $6::text = 'asc' THEN metric_sortino END ASC NULLS LAST,
  CASE WHEN $5::text = 'cagr' AND $6::text = 'desc' THEN metric_cagr END DESC NULLS LAST,
  CASE WHEN $5::text = 'cagr' AND $6::text = 'asc' THEN metric_cagr END ASC NULLS LAST,
  CASE WHEN $5::text = 'maxDrawdown' AND $6::text = 'desc' THEN metric_max_drawdown END DESC NULLS LAST,
  CASE WHEN $5::text = 'maxDrawdown' AND $6::text = 'asc' THEN metric_max_drawdown END ASC NULLS LAST,
  CASE WHEN $5::text = 'durationMs' AND $6::text = 'desc' THEN metric_duration_ms END DESC NULLS LAST,
  CASE WHEN $5::text = 'durationMs' AND $6::text = 'asc' THEN metric_duration_ms END ASC NULLS LAST,
  CASE WHEN $5::text = 'status' AND $6::text = 'desc' THEN status END DESC,
  CASE WHEN $5::text = 'status' AND $6::text = 'asc' THEN status END ASC,
  CASE WHEN $5::text = 'symbol' AND $6::text = 'desc' THEN symbol END DESC,
  CASE WHEN $5::text = 'symbol' AND $6::text = 'asc' THEN symbol END ASC,
  CASE WHEN $5::text = 'config' AND $6::text = 'desc' THEN playbook END DESC,
  CASE WHEN $5::text = 'config' AND $6::text = 'asc' THEN playbook END ASC,
  CASE WHEN $5::text = 'monthlyReturnAvg' AND $6::text = 'desc' THEN metric_monthly_return_avg END DESC NULLS LAST,
  CASE WHEN $5::text = 'monthlyReturnAvg' AND $6::text = 'asc' THEN metric_monthly_return_avg END ASC NULLS LAST,
  CASE WHEN $5::text = 'monthlyReturnMed' AND $6::text = 'desc' THEN metric_monthly_return_med END DESC NULLS LAST,
  CASE WHEN $5::text = 'monthlyReturnMed' AND $6::text = 'asc' THEN metric_monthly_return_med END ASC NULLS LAST,
  CASE WHEN $5::text = 'yearlyReturnAvg' AND $6::text = 'desc' THEN metric_yearly_return_avg END DESC NULLS LAST,
  CASE WHEN $5::text = 'yearlyReturnAvg' AND $6::text = 'asc' THEN metric_yearly_return_avg END ASC NULLS LAST,
  CASE WHEN $5::text = 'yearlyReturnMed' AND $6::text = 'desc' THEN metric_yearly_return_med END DESC NULLS LAST,
  CASE WHEN $5::text = 'yearlyReturnMed' AND $6::text = 'asc' THEN metric_yearly_return_med END ASC NULLS LAST,
  CASE WHEN $5::text = 'tradesPerMonthAvg' AND $6::text = 'desc' THEN metric_trades_per_month_avg END DESC NULLS LAST,
  CASE WHEN $5::text = 'tradesPerMonthAvg' AND $6::text = 'asc' THEN metric_trades_per_month_avg END ASC NULLS LAST,
  CASE WHEN $5::text = 'tradesPerMonthMed' AND $6::text = 'desc' THEN metric_trades_per_month_med END DESC NULLS LAST,
  CASE WHEN $5::text = 'tradesPerMonthMed' AND $6::text = 'asc' THEN metric_trades_per_month_med END ASC NULLS LAST,
  CASE WHEN $5::text = 'winRate' AND $6::text = 'desc' THEN metric_win_rate END DESC NULLS LAST,
  CASE WHEN $5::text = 'winRate' AND $6::text = 'asc' THEN metric_win_rate END ASC NULLS LAST,
  CASE WHEN $5::text = 'monthlyWinRateAvg' AND $6::text = 'desc' THEN metric_monthly_win_rate_avg END DESC NULLS LAST,
  CASE WHEN $5::text = 'monthlyWinRateAvg' AND $6::text = 'asc' THEN metric_monthly_win_rate_avg END ASC NULLS LAST,
  CASE WHEN $5::text = 'monthlyWinRateMed' AND $6::text = 'desc' THEN metric_monthly_win_rate_med END DESC NULLS LAST,
  CASE WHEN $5::text = 'monthlyWinRateMed' AND $6::text = 'asc' THEN metric_monthly_win_rate_med END ASC NULLS LAST,
  CASE WHEN $5::text = 'yearlyWinRateAvg' AND $6::text = 'desc' THEN metric_yearly_win_rate_avg END DESC NULLS LAST,
  CASE WHEN $5::text = 'yearlyWinRateAvg' AND $6::text = 'asc' THEN metric_yearly_win_rate_avg END ASC NULLS LAST,
  CASE WHEN $5::text = 'yearlyWinRateMed' AND $6::text = 'desc' THEN metric_yearly_win_rate_med END DESC NULLS LAST,
  CASE WHEN $5::text = 'yearlyWinRateMed' AND $6::text = 'asc' THEN metric_yearly_win_rate_med END ASC NULLS LAST,
  CASE WHEN $5::text = 'label' AND $6::text = 'desc' THEN label END DESC,
  CASE WHEN $5::text = 'label' AND $6::text = 'asc' THEN label END ASC,
  label ASC,
  id ASC
LIMIT $7 OFFSET $8;

-- name: MarkInstancesRunning :exec
UPDATE experiment_instance
SET status = 'running', error = ''
WHERE experiment_id = $1;

-- name: MarkRunningInstancesCancelled :exec
UPDATE experiment_instance
SET status = 'cancelled',
    error = CASE WHEN status = 'running' THEN 'cancelled' ELSE error END
WHERE experiment_id = $1;

-- name: ListInstancesByExperiment :many
SELECT *
FROM experiment_instance
WHERE experiment_id = $1
ORDER BY label ASC, id ASC;

-- name: ListInstancesByIDs :many
SELECT *
FROM experiment_instance
WHERE experiment_id = $1 AND id = ANY($2::uuid[]);

-- name: UpdateInstanceCompleted :exec
UPDATE experiment_instance
SET status = 'completed',
    error = '',
    started_at_ms = $3,
    finished_at_ms = $4
WHERE experiment_id = $1 AND id = $2;

-- name: UpdateInstanceFailed :exec
UPDATE experiment_instance
SET status = 'failed',
    error = $3,
    started_at_ms = $4,
    finished_at_ms = $5
WHERE experiment_id = $1 AND id = $2;

-- name: UpdateInstanceMetrics :exec
UPDATE experiment_instance
SET metric_duration_ms = $3,
    metric_cagr = $4,
    metric_max_drawdown = $5,
    metric_sharpe = $6,
    metric_sortino = $7,
    metric_monthly_return_avg = $8,
    metric_monthly_return_med = $9,
    metric_yearly_return_avg = $10,
    metric_yearly_return_med = $11,
    metric_trades_per_month_avg = $12,
    metric_trades_per_month_med = $13,
    metric_wins = $14,
    metric_losses = $15,
    metric_breakeven = $16,
    metric_monthly_win_rate_avg = $17,
    metric_monthly_win_rate_med = $18,
    metric_yearly_win_rate_avg = $19,
    metric_yearly_win_rate_med = $20
WHERE experiment_id = $1 AND id = $2;

-- name: MarkFailedInstancesRunning :exec
UPDATE experiment_instance
SET status = 'running',
    error = ''
WHERE experiment_id = $1 AND status = 'failed';

-- name: CountFailedInstancesByExperiment :one
SELECT COUNT(*)::bigint AS n
FROM experiment_instance
WHERE experiment_id = $1 AND status = 'failed';
