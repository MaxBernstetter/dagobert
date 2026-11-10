-- Drop "experiment_instance_period_stat" table: per-instance period rows are instance data, kept in
-- each instance's period_stats.parquet. Postgres holds experiment metadata and aggregates only.
DROP TABLE "experiment_instance_period_stat";
