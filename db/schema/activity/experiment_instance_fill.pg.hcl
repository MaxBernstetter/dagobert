table "experiment_instance_fill" {
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
  column "action_id" {
    null = true
    type = uuid
  }
  column "symbol" {
    null = false
    type = text
  }
  column "ts_ms" {
    null = false
    type = bigint
  }
  # buy | sell
  column "side" {
    null = false
    type = text
  }
  column "qty" {
    null = false
    type = double_precision
  }
  column "price" {
    null = false
    type = double_precision
  }
  column "fee" {
    null = false
    type = double_precision
    default = 0
  }
  column "fee_asset" {
    null = false
    type = text
    default = ""
  }

  primary_key {
    columns = [column.id]
  }
  foreign_key "experiment_instance_fill_experiment_id_fk" {
    columns     = [column.experiment_id]
    ref_columns = [table.experiment.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
  foreign_key "experiment_instance_fill_instance_id_fk" {
    columns     = [column.instance_id]
    ref_columns = [table.experiment_instance.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
  foreign_key "experiment_instance_fill_action_id_fk" {
    columns     = [column.action_id]
    ref_columns = [table.experiment_instance_action.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }
  index "experiment_instance_fill_instance_ts_idx" {
    columns = [column.instance_id, column.ts_ms]
  }
}
