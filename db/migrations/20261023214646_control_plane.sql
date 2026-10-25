-- Modify "study_instance" table
ALTER TABLE "study_instance" ADD COLUMN "label" text NOT NULL DEFAULT '', ADD COLUMN "config_hash" text NOT NULL, ADD COLUMN "playbook_key" text NOT NULL, ADD COLUMN "workspace_rel_path" text NOT NULL, ADD COLUMN "error" text NOT NULL DEFAULT '', ADD COLUMN "started_at_ms" bigint NULL, ADD COLUMN "finished_at_ms" bigint NULL, ADD COLUMN "cfg_fast_hr" integer NULL, ADD COLUMN "cfg_slow_hr" integer NULL, ADD COLUMN "cfg_rsi_bull" double precision NULL, ADD COLUMN "cfg_rsi_bear" double precision NULL, ADD COLUMN "cfg_vol_ratio_bull" double precision NULL, ADD COLUMN "cfg_vol_ratio_bear" double precision NULL, ADD COLUMN "cfg_atr_stop_mult" double precision NULL, ADD COLUMN "cfg_min_bars_flat_after_sell" integer NULL, ADD COLUMN "cfg_min_bars_long_before_sell" integer NULL, ADD COLUMN "metric_duration_ms" bigint NULL, ADD COLUMN "metric_cagr" double precision NULL, ADD COLUMN "metric_max_drawdown" double precision NULL, ADD COLUMN "metric_sortino" double precision NULL, ADD COLUMN "metric_monthly_return_avg" double precision NULL, ADD COLUMN "metric_monthly_return_med" double precision NULL, ADD COLUMN "metric_yearly_return_avg" double precision NULL, ADD COLUMN "metric_yearly_return_med" double precision NULL, ADD COLUMN "metric_trades_per_month_avg" double precision NULL, ADD COLUMN "metric_trades_per_month_med" double precision NULL;
-- Create index "study_instance_study_id_cagr_idx" to table: "study_instance"
CREATE INDEX "study_instance_study_id_cagr_idx" ON "study_instance" ("study_id", "metric_cagr");
-- Create index "study_instance_study_id_duration_idx" to table: "study_instance"
CREATE INDEX "study_instance_study_id_duration_idx" ON "study_instance" ("study_id", "metric_duration_ms");
-- Create index "study_instance_study_id_idx" to table: "study_instance"
CREATE INDEX "study_instance_study_id_idx" ON "study_instance" ("study_id");
-- Create index "study_instance_study_id_maxdd_idx" to table: "study_instance"
CREATE INDEX "study_instance_study_id_maxdd_idx" ON "study_instance" ("study_id", "metric_max_drawdown");
-- Create index "study_instance_study_id_mo_ret_idx" to table: "study_instance"
CREATE INDEX "study_instance_study_id_mo_ret_idx" ON "study_instance" ("study_id", "metric_monthly_return_avg");
-- Create index "study_instance_study_id_sortino_idx" to table: "study_instance"
CREATE INDEX "study_instance_study_id_sortino_idx" ON "study_instance" ("study_id", "metric_sortino");
-- Create index "study_instance_study_id_status_idx" to table: "study_instance"
CREATE INDEX "study_instance_study_id_status_idx" ON "study_instance" ("study_id", "status");
-- Create index "study_instance_study_id_symbol_idx" to table: "study_instance"
CREATE INDEX "study_instance_study_id_symbol_idx" ON "study_instance" ("study_id", "symbol");
-- Modify "study" table
ALTER TABLE "study" ADD COLUMN "error" text NOT NULL DEFAULT '', ADD COLUMN "provider" text NOT NULL DEFAULT 'binance', ADD COLUMN "base" text NOT NULL DEFAULT 'BTC', ADD COLUMN "quote" text NOT NULL DEFAULT 'USDT', ADD COLUMN "from_ms" bigint NOT NULL, ADD COLUMN "to_ms" bigint NOT NULL, ADD COLUMN "initial_usdc" double precision NOT NULL DEFAULT 0, ADD COLUMN "initial_usdt" double precision NOT NULL DEFAULT 0, ADD COLUMN "initial_base" double precision NOT NULL DEFAULT 0, ADD COLUMN "target_weight_pct" double precision NOT NULL DEFAULT 0, ADD COLUMN "cash_reserve_quote" double precision NOT NULL DEFAULT 0, ADD COLUMN "min_order_quote" double precision NOT NULL DEFAULT 0, ADD COLUMN "redownload_missing" boolean NOT NULL DEFAULT false, ADD COLUMN "algo" text NOT NULL DEFAULT 'v1', ADD COLUMN "spread_bps" double precision NOT NULL DEFAULT 0, ADD COLUMN "slippage_bps" double precision NOT NULL DEFAULT 0, ADD COLUMN "fee_bps" double precision NOT NULL DEFAULT 0, ADD COLUMN "started_at_ms" bigint NULL, ADD COLUMN "finished_at_ms" bigint NULL, ADD COLUMN "progress_stage" integer NOT NULL DEFAULT 0, ADD COLUMN "progress_stages" integer NOT NULL DEFAULT 0, ADD COLUMN "progress_stage_label" text NOT NULL DEFAULT '', ADD COLUMN "progress_instance_done" integer NOT NULL DEFAULT 0, ADD COLUMN "progress_instance_total" integer NOT NULL DEFAULT 0;
-- Create index "study_status_idx" to table: "study"
CREATE INDEX "study_status_idx" ON "study" ("status");
-- Create "study_instance_action" table
CREATE TABLE "study_instance_action" (
  "id" uuid NOT NULL,
  "study_id" uuid NOT NULL,
  "instance_id" uuid NOT NULL,
  "symbol" text NOT NULL,
  "bar_open_ms" bigint NOT NULL,
  "side" text NOT NULL,
  "weight_pct" double precision NULL,
  "reason" text NOT NULL DEFAULT '',
  PRIMARY KEY ("id"),
  CONSTRAINT "study_instance_action_instance_id_fk" FOREIGN KEY ("instance_id") REFERENCES "study_instance" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "study_instance_action_study_id_fk" FOREIGN KEY ("study_id") REFERENCES "study" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "study_instance_action_instance_bar_idx" to table: "study_instance_action"
CREATE INDEX "study_instance_action_instance_bar_idx" ON "study_instance_action" ("instance_id", "bar_open_ms");
-- Create index "study_instance_action_study_id_idx" to table: "study_instance_action"
CREATE INDEX "study_instance_action_study_id_idx" ON "study_instance_action" ("study_id");
-- Create "study_instance_fill" table
CREATE TABLE "study_instance_fill" (
  "id" uuid NOT NULL,
  "study_id" uuid NOT NULL,
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
  CONSTRAINT "study_instance_fill_action_id_fk" FOREIGN KEY ("action_id") REFERENCES "study_instance_action" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  CONSTRAINT "study_instance_fill_instance_id_fk" FOREIGN KEY ("instance_id") REFERENCES "study_instance" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "study_instance_fill_study_id_fk" FOREIGN KEY ("study_id") REFERENCES "study" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "study_instance_fill_instance_ts_idx" to table: "study_instance_fill"
CREATE INDEX "study_instance_fill_instance_ts_idx" ON "study_instance_fill" ("instance_id", "ts_ms");
-- Create "study_instance_period_stat" table
CREATE TABLE "study_instance_period_stat" (
  "instance_id" uuid NOT NULL,
  "period_kind" text NOT NULL,
  "period_key" text NOT NULL,
  "return_pct" double precision NULL,
  "trade_count" integer NULL,
  PRIMARY KEY ("instance_id", "period_kind", "period_key"),
  CONSTRAINT "study_instance_period_stat_instance_id_fk" FOREIGN KEY ("instance_id") REFERENCES "study_instance" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "study_instance_period_stat_kind_idx" to table: "study_instance_period_stat"
CREATE INDEX "study_instance_period_stat_kind_idx" ON "study_instance_period_stat" ("instance_id", "period_kind");
