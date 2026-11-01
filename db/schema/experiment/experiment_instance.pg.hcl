table "experiment_instance" {
  schema = schema.public

  column "id" {
    null = false
    type = uuid
  }
  column "experiment_id" {
    null = false
    type = uuid
  }
  column "label" {
    null = false
    type = text
    default = ""
  }
  column "playbook" {
    null = false
    type = text
  }
  column "symbol" {
    null = false
    type = text
  }
  column "config_hash" {
    null = false
    type = text
    default = ""
  }
  column "playbook_key" {
    null = false
    type = text
    default = ""
  }
  column "workspace_rel_path" {
    null = false
    type = text
    default = ""
  }
  column "status" {
    null = false
    type = text
  }
  column "error" {
    null = false
    type = text
    default = ""
  }
  column "started_at_ms" {
    null = true
    type = bigint
  }
  column "finished_at_ms" {
    null = true
    type = bigint
  }

  # ma-cross strategy params (null when other playbook).
  column "cfg_fast_hr" {
    null = true
    type = integer
  }
  column "cfg_slow_hr" {
    null = true
    type = integer
  }
  # signal-consensus strategy params.
  column "cfg_rsi_bull" {
    null = true
    type = double_precision
  }
  column "cfg_rsi_bear" {
    null = true
    type = double_precision
  }
  column "cfg_vol_ratio_bull" {
    null = true
    type = double_precision
  }
  column "cfg_vol_ratio_bear" {
    null = true
    type = double_precision
  }
  column "cfg_atr_stop_mult" {
    null = true
    type = double_precision
  }
  column "cfg_min_bars_flat_after_sell" {
    null = true
    type = integer
  }
  column "cfg_min_bars_long_before_sell" {
    null = true
    type = integer
  }
  # Canonical strategy config as JSON (sizing included); NULL on instances made before it was stored.
  column "strategy_config_json" {
    null = true
    type = text
  }

  # Denormalized list / sort metrics (UI table).
  column "metric_duration_ms" {
    null = true
    type = bigint
  }
  column "metric_cagr" {
    null = true
    type = double_precision
  }
  column "metric_max_drawdown" {
    null = true
    type = double_precision
  }
  column "metric_sharpe" {
    null = true
    type = double_precision
  }
  column "metric_sortino" {
    null = true
    type = double_precision
  }
  column "metric_monthly_return_avg" {
    null = true
    type = double_precision
  }
  column "metric_monthly_return_med" {
    null = true
    type = double_precision
  }
  column "metric_yearly_return_avg" {
    null = true
    type = double_precision
  }
  column "metric_yearly_return_med" {
    null = true
    type = double_precision
  }
  column "metric_trades_per_month_avg" {
    null = true
    type = double_precision
  }
  column "metric_trades_per_month_med" {
    null = true
    type = double_precision
  }
  column "metric_wins" {
    null = true
    type = integer
  }
  column "metric_losses" {
    null = true
    type = integer
  }
  column "metric_breakeven" {
    null = true
    type = integer
  }
  column "metric_win_rate" {
    null    = true
    type    = real
    as {
      expr = "CASE WHEN (COALESCE(metric_wins, 0) + COALESCE(metric_losses, 0) + COALESCE(metric_breakeven, 0)) = 0 THEN NULL ELSE metric_wins::real / (COALESCE(metric_wins, 0) + COALESCE(metric_losses, 0) + COALESCE(metric_breakeven, 0))::real END"
      type = STORED
    }
  }
  column "metric_monthly_win_rate_avg" {
    null = true
    type = real
  }
  column "metric_monthly_win_rate_med" {
    null = true
    type = real
  }
  column "metric_yearly_win_rate_avg" {
    null = true
    type = real
  }
  column "metric_yearly_win_rate_med" {
    null = true
    type = real
  }

  primary_key {
    columns = [column.id]
  }
  foreign_key "experiment_instance_experiment_id_fk" {
    columns     = [column.experiment_id]
    ref_columns = [table.experiment.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
  index "experiment_instance_experiment_id_idx" {
    columns = [column.experiment_id]
  }
  index "experiment_instance_experiment_id_status_idx" {
    columns = [column.experiment_id, column.status]
  }
  index "experiment_instance_experiment_id_symbol_idx" {
    columns = [column.experiment_id, column.symbol]
  }
  index "experiment_instance_experiment_id_sharpe_idx" {
    columns = [column.experiment_id, column.metric_sharpe]
  }
  index "experiment_instance_experiment_id_sortino_idx" {
    columns = [column.experiment_id, column.metric_sortino]
  }
  index "experiment_instance_experiment_id_cagr_idx" {
    columns = [column.experiment_id, column.metric_cagr]
  }
  index "experiment_instance_experiment_id_maxdd_idx" {
    columns = [column.experiment_id, column.metric_max_drawdown]
  }
  index "experiment_instance_experiment_id_duration_idx" {
    columns = [column.experiment_id, column.metric_duration_ms]
  }
  index "experiment_instance_experiment_id_mo_ret_idx" {
    columns = [column.experiment_id, column.metric_monthly_return_avg]
  }
  index "experiment_instance_experiment_id_win_rate_idx" {
    columns = [column.experiment_id, column.metric_win_rate]
  }
}
