# Chart markers / Activity working memory (not lake documents).
table "experiment_instance_action" {
  schema = schema.public

  column "id" {
    null = false
    type = uuid
  }
  column "experiment_id" {
    null = false
    type = uuid
  }
  column "instance_id" {
    null = false
    type = uuid
  }
  column "symbol" {
    null = false
    type = text
  }
  # bar open time (ms) for chart alignment
  column "bar_open_ms" {
    null = false
    type = bigint
  }
  # buy | sell | flat | other
  column "side" {
    null = false
    type = text
  }
  column "weight_pct" {
    null = true
    type = double_precision
  }
  column "reason" {
    null = false
    type = text
    default = ""
  }

  primary_key {
    columns = [column.id]
  }
  foreign_key "experiment_instance_action_experiment_id_fk" {
    columns     = [column.experiment_id]
    ref_columns = [table.experiment.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
  foreign_key "experiment_instance_action_instance_id_fk" {
    columns     = [column.instance_id]
    ref_columns = [table.experiment_instance.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
  index "experiment_instance_action_instance_bar_idx" {
    columns = [column.instance_id, column.bar_open_ms]
  }
  index "experiment_instance_action_experiment_id_idx" {
    columns = [column.experiment_id]
  }
}
