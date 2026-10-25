# Normalized period returns / trade counts (replaces report.json period blobs for UI).
table "experiment_instance_period_stat" {
  schema = schema.public

  column "instance_id" {
    null = false
    type = uuid
  }
  # monthly | yearly
  column "period_kind" {
    null = false
    type = text
  }
  # e.g. 2024-01 or 2024
  column "period_key" {
    null = false
    type = text
  }
  column "return_pct" {
    null = true
    type = double_precision
  }
  column "trade_count" {
    null = true
    type = integer
  }
  column "win_count" {
    null = true
    type = integer
  }

  primary_key {
    columns = [column.instance_id, column.period_kind, column.period_key]
  }
  foreign_key "experiment_instance_period_stat_instance_id_fk" {
    columns     = [column.instance_id]
    ref_columns = [table.experiment_instance.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
  index "experiment_instance_period_stat_kind_idx" {
    columns = [column.instance_id, column.period_kind]
  }
}
