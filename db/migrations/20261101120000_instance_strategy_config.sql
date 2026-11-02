-- Modify "experiment_instance" table: canonical strategy config as JSON. Postgres instances
-- stored only the cfg_* columns, which cannot hold sizing; NULL means "use the cfg_* columns".
ALTER TABLE "experiment_instance" ADD COLUMN "strategy_config_json" text NULL;
