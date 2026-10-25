-- Win rate facts, trade samples, and the experiment reduction.
ALTER TABLE "experiment_instance" ADD COLUMN "metric_wins" integer NULL;
ALTER TABLE "experiment_instance" ADD COLUMN "metric_losses" integer NULL;
ALTER TABLE "experiment_instance" ADD COLUMN "metric_breakeven" integer NULL;
ALTER TABLE "experiment_instance" ADD COLUMN "metric_win_rate" real GENERATED ALWAYS AS (
  CASE
    WHEN (COALESCE(metric_wins, 0) + COALESCE(metric_losses, 0) + COALESCE(metric_breakeven, 0)) = 0 THEN NULL
    ELSE metric_wins::real / (COALESCE(metric_wins, 0) + COALESCE(metric_losses, 0) + COALESCE(metric_breakeven, 0))::real
  END
) STORED;
ALTER TABLE "experiment_instance" ADD COLUMN "metric_monthly_win_rate_avg" real NULL;
ALTER TABLE "experiment_instance" ADD COLUMN "metric_monthly_win_rate_med" real NULL;
ALTER TABLE "experiment_instance" ADD COLUMN "metric_yearly_win_rate_avg" real NULL;
ALTER TABLE "experiment_instance" ADD COLUMN "metric_yearly_win_rate_med" real NULL;
CREATE INDEX "experiment_instance_experiment_id_win_rate_idx" ON "experiment_instance" ("experiment_id", "metric_win_rate");

ALTER TABLE "experiment_instance_period_stat" ADD COLUMN "win_count" integer NULL;

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

REVOKE ALL ON TABLE public.experiment_performance FROM anon, authenticated, PUBLIC;
REVOKE ALL ON TABLE public.experiment_period_stat FROM anon, authenticated, PUBLIC;
REVOKE ALL ON TABLE public.experiment_outcome_stat FROM anon, authenticated, PUBLIC;
REVOKE ALL ON TABLE public.experiment_trade_outcome FROM anon, authenticated, PUBLIC;
