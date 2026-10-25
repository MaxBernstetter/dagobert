-- Create "experiment" table
CREATE TABLE "experiment" (
  "id" uuid NOT NULL,
  "name" text NOT NULL,
  "description" text NOT NULL DEFAULT '',
  "status" text NOT NULL,
  "error" text NOT NULL DEFAULT '',
  "provider" text NOT NULL DEFAULT 'binance',
  "base" text NOT NULL DEFAULT 'BTC',
  "quote" text NOT NULL DEFAULT 'USDT',
  "from_ms" bigint NOT NULL DEFAULT 0,
  "to_ms" bigint NOT NULL DEFAULT 0,
  "initial_usdc" double precision NOT NULL DEFAULT 0,
  "initial_usdt" double precision NOT NULL DEFAULT 0,
  "initial_base" double precision NOT NULL DEFAULT 0,
  "target_weight_pct" double precision NOT NULL DEFAULT 0,
  "cash_reserve_quote" double precision NOT NULL DEFAULT 0,
  "min_order_quote" double precision NOT NULL DEFAULT 0,
  "redownload_missing" boolean NOT NULL DEFAULT false,
  "algo" text NOT NULL DEFAULT 'v1',
  "spread_bps" double precision NOT NULL DEFAULT 0,
  "slippage_bps" double precision NOT NULL DEFAULT 0,
  "fee_bps" double precision NOT NULL DEFAULT 0,
  "created_at_ms" bigint NOT NULL,
  "started_at_ms" bigint NULL,
  "finished_at_ms" bigint NULL,
  "progress_stage" integer NOT NULL DEFAULT 0,
  "progress_stages" integer NOT NULL DEFAULT 0,
  "progress_stage_label" text NOT NULL DEFAULT '',
  "progress_instance_done" integer NOT NULL DEFAULT 0,
  "progress_instance_total" integer NOT NULL DEFAULT 0,
  PRIMARY KEY ("id")
);
-- Create index "experiment_created_at_ms_idx" to table: "experiment"
CREATE INDEX "experiment_created_at_ms_idx" ON "experiment" ("created_at_ms");
-- Create index "experiment_status_idx" to table: "experiment"
CREATE INDEX "experiment_status_idx" ON "experiment" ("status");
-- Create "experiment_instance" table
CREATE TABLE "experiment_instance" (
  "id" uuid NOT NULL,
  "experiment_id" uuid NOT NULL,
  "label" text NOT NULL DEFAULT '',
  "playbook" text NOT NULL,
  "symbol" text NOT NULL,
  "config_hash" text NOT NULL DEFAULT '',
  "playbook_key" text NOT NULL DEFAULT '',
  "workspace_rel_path" text NOT NULL DEFAULT '',
  "status" text NOT NULL,
  "error" text NOT NULL DEFAULT '',
  "started_at_ms" bigint NULL,
  "finished_at_ms" bigint NULL,
  "cfg_fast_hr" integer NULL,
  "cfg_slow_hr" integer NULL,
  "cfg_rsi_bull" double precision NULL,
  "cfg_rsi_bear" double precision NULL,
  "cfg_vol_ratio_bull" double precision NULL,
  "cfg_vol_ratio_bear" double precision NULL,
  "cfg_atr_stop_mult" double precision NULL,
  "cfg_min_bars_flat_after_sell" integer NULL,
  "cfg_min_bars_long_before_sell" integer NULL,
  "metric_duration_ms" bigint NULL,
  "metric_cagr" double precision NULL,
  "metric_max_drawdown" double precision NULL,
  "metric_sharpe" double precision NULL,
  "metric_sortino" double precision NULL,
  "metric_monthly_return_avg" double precision NULL,
  "metric_monthly_return_med" double precision NULL,
  "metric_yearly_return_avg" double precision NULL,
  "metric_yearly_return_med" double precision NULL,
  "metric_trades_per_month_avg" double precision NULL,
  "metric_trades_per_month_med" double precision NULL,
  "metric_wins" integer NULL,
  "metric_losses" integer NULL,
  "metric_breakeven" integer NULL,
  "metric_win_rate" real GENERATED ALWAYS AS (
    CASE
      WHEN (COALESCE(metric_wins, 0) + COALESCE(metric_losses, 0) + COALESCE(metric_breakeven, 0)) = 0 THEN NULL
      ELSE metric_wins::real / (COALESCE(metric_wins, 0) + COALESCE(metric_losses, 0) + COALESCE(metric_breakeven, 0))::real
    END
  ) STORED,
  "metric_monthly_win_rate_avg" real NULL,
  "metric_monthly_win_rate_med" real NULL,
  "metric_yearly_win_rate_avg" real NULL,
  "metric_yearly_win_rate_med" real NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "experiment_instance_experiment_id_fk" FOREIGN KEY ("experiment_id") REFERENCES "experiment" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "experiment_instance_experiment_id_cagr_idx" to table: "experiment_instance"
CREATE INDEX "experiment_instance_experiment_id_cagr_idx" ON "experiment_instance" ("experiment_id", "metric_cagr");
-- Create index "experiment_instance_experiment_id_duration_idx" to table: "experiment_instance"
CREATE INDEX "experiment_instance_experiment_id_duration_idx" ON "experiment_instance" ("experiment_id", "metric_duration_ms");
-- Create index "experiment_instance_experiment_id_idx" to table: "experiment_instance"
CREATE INDEX "experiment_instance_experiment_id_idx" ON "experiment_instance" ("experiment_id");
-- Create index "experiment_instance_experiment_id_maxdd_idx" to table: "experiment_instance"
CREATE INDEX "experiment_instance_experiment_id_maxdd_idx" ON "experiment_instance" ("experiment_id", "metric_max_drawdown");
-- Create index "experiment_instance_experiment_id_mo_ret_idx" to table: "experiment_instance"
CREATE INDEX "experiment_instance_experiment_id_mo_ret_idx" ON "experiment_instance" ("experiment_id", "metric_monthly_return_avg");
CREATE INDEX "experiment_instance_experiment_id_win_rate_idx" ON "experiment_instance" ("experiment_id", "metric_win_rate");
-- Create index "experiment_instance_experiment_id_sharpe_idx" to table: "experiment_instance"
CREATE INDEX "experiment_instance_experiment_id_sharpe_idx" ON "experiment_instance" ("experiment_id", "metric_sharpe");
-- Create index "experiment_instance_experiment_id_sortino_idx" to table: "experiment_instance"
CREATE INDEX "experiment_instance_experiment_id_sortino_idx" ON "experiment_instance" ("experiment_id", "metric_sortino");
-- Create index "experiment_instance_experiment_id_status_idx" to table: "experiment_instance"
CREATE INDEX "experiment_instance_experiment_id_status_idx" ON "experiment_instance" ("experiment_id", "status");
-- Create index "experiment_instance_experiment_id_symbol_idx" to table: "experiment_instance"
CREATE INDEX "experiment_instance_experiment_id_symbol_idx" ON "experiment_instance" ("experiment_id", "symbol");
-- Create "experiment_instance_action" table
CREATE TABLE "experiment_instance_action" (
  "id" uuid NOT NULL,
  "experiment_id" uuid NOT NULL,
  "instance_id" uuid NOT NULL,
  "symbol" text NOT NULL,
  "bar_open_ms" bigint NOT NULL,
  "side" text NOT NULL,
  "weight_pct" double precision NULL,
  "reason" text NOT NULL DEFAULT '',
  PRIMARY KEY ("id"),
  CONSTRAINT "experiment_instance_action_instance_id_fk" FOREIGN KEY ("instance_id") REFERENCES "experiment_instance" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "experiment_instance_action_experiment_id_fk" FOREIGN KEY ("experiment_id") REFERENCES "experiment" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "experiment_instance_action_instance_bar_idx" to table: "experiment_instance_action"
CREATE INDEX "experiment_instance_action_instance_bar_idx" ON "experiment_instance_action" ("instance_id", "bar_open_ms");
-- Create index "experiment_instance_action_experiment_id_idx" to table: "experiment_instance_action"
CREATE INDEX "experiment_instance_action_experiment_id_idx" ON "experiment_instance_action" ("experiment_id");
-- Create "experiment_instance_fill" table
CREATE TABLE "experiment_instance_fill" (
  "id" uuid NOT NULL,
  "experiment_id" uuid NOT NULL,
  "instance_id" uuid NOT NULL,
  "action_id" uuid NULL,
  "symbol" text NOT NULL,
  "ts_ms" bigint NOT NULL,
  "side" text NOT NULL,
  "qty" double precision NOT NULL,
  "price" double precision NOT NULL,
  "fee" double precision NOT NULL DEFAULT 0,
  "fee_asset" text NOT NULL DEFAULT '',
  PRIMARY KEY ("id"),
  CONSTRAINT "experiment_instance_fill_action_id_fk" FOREIGN KEY ("action_id") REFERENCES "experiment_instance_action" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  CONSTRAINT "experiment_instance_fill_instance_id_fk" FOREIGN KEY ("instance_id") REFERENCES "experiment_instance" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "experiment_instance_fill_experiment_id_fk" FOREIGN KEY ("experiment_id") REFERENCES "experiment" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "experiment_instance_fill_instance_ts_idx" to table: "experiment_instance_fill"
CREATE INDEX "experiment_instance_fill_instance_ts_idx" ON "experiment_instance_fill" ("instance_id", "ts_ms");
-- Create "experiment_instance_period_stat" table
CREATE TABLE "experiment_instance_period_stat" (
  "instance_id" uuid NOT NULL,
  "period_kind" text NOT NULL,
  "period_key" text NOT NULL,
  "return_pct" double precision NULL,
  "trade_count" integer NULL,
  "win_count" integer NULL,
  PRIMARY KEY ("instance_id", "period_kind", "period_key"),
  CONSTRAINT "experiment_instance_period_stat_instance_id_fk" FOREIGN KEY ("instance_id") REFERENCES "experiment_instance" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "experiment_instance_period_stat_kind_idx" to table: "experiment_instance_period_stat"
CREATE INDEX "experiment_instance_period_stat_kind_idx" ON "experiment_instance_period_stat" ("instance_id", "period_kind");

CREATE TABLE "experiment_performance" (
  "experiment_id" uuid NOT NULL,
  "wins" integer NOT NULL,
  "losses" integer NOT NULL,
  "breakeven" integer NOT NULL,
  "win_rate" real GENERATED ALWAYS AS (
    CASE
      WHEN (wins + losses + breakeven) = 0 THEN NULL
      ELSE wins::real / (wins + losses + breakeven)::real
    END
  ) STORED,
  PRIMARY KEY ("experiment_id"),
  CONSTRAINT "experiment_performance_experiment_id_fk" FOREIGN KEY ("experiment_id") REFERENCES "experiment" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);

CREATE TABLE "experiment_period_stat" (
  "experiment_id" uuid NOT NULL,
  "period_kind" text NOT NULL,
  "period_key" text NOT NULL,
  "wins" integer NOT NULL,
  "trades" integer NOT NULL,
  PRIMARY KEY ("experiment_id", "period_kind", "period_key"),
  CONSTRAINT "experiment_period_stat_experiment_id_fk" FOREIGN KEY ("experiment_id") REFERENCES "experiment" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);

CREATE TABLE "experiment_outcome_stat" (
  "experiment_id" uuid NOT NULL,
  "side" smallint NOT NULL,
  "unit" smallint NOT NULL,
  "average" real NOT NULL,
  "p10" real NOT NULL,
  "p25" real NOT NULL,
  "p50" real NOT NULL,
  "p75" real NOT NULL,
  "p90" real NOT NULL,
  "p95" real NOT NULL,
  PRIMARY KEY ("experiment_id", "side", "unit"),
  CONSTRAINT "experiment_outcome_stat_experiment_id_fk" FOREIGN KEY ("experiment_id") REFERENCES "experiment" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);

CREATE TABLE "experiment_trade_outcome" (
  "instance_id" uuid NOT NULL,
  "ordinal" integer NOT NULL,
  "side" smallint NOT NULL,
  "quote" real NOT NULL,
  "percent" real NULL,
  PRIMARY KEY ("instance_id", "ordinal"),
  CONSTRAINT "experiment_trade_outcome_instance_id_fk" FOREIGN KEY ("instance_id") REFERENCES "experiment_instance" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
