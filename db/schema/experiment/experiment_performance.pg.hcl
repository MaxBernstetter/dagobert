# One row per experiment: summed closed-trade counts. Win rate is generated.
table "experiment_performance" {
  schema = schema.public

  column "experiment_id" {
    null = false
    type = uuid
  }
  column "wins" {
    null = false
    type = integer
  }
  column "losses" {
    null = false
    type = integer
  }
  column "breakeven" {
    null = false
    type = integer
  }
  column "win_rate" {
    null = true
    type = real
    as {
      expr = "CASE WHEN (wins + losses + breakeven) = 0 THEN NULL ELSE wins::real / (wins + losses + breakeven)::real END"
      type = STORED
    }
  }

  primary_key {
    columns = [column.experiment_id]
  }
  foreign_key "experiment_performance_experiment_id_fk" {
    columns     = [column.experiment_id]
    ref_columns = [table.experiment.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
}

# Rolled-up calendar buckets. Rate is wins / trades in Go.
table "experiment_period_stat" {
  schema = schema.public

  column "experiment_id" {
    null = false
    type = uuid
  }
  column "period_kind" {
    null = false
    type = text
  }
  column "period_key" {
    null = false
    type = text
  }
  column "wins" {
    null = false
    type = integer
  }
  column "trades" {
    null = false
    type = integer
  }

  primary_key {
    columns = [column.experiment_id, column.period_kind, column.period_key]
  }
  foreign_key "experiment_period_stat_experiment_id_fk" {
    columns     = [column.experiment_id]
    ref_columns = [table.experiment.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
}

# Stored experiment ladder. Four rows: side gain/loss × unit quote/percent.
table "experiment_outcome_stat" {
  schema = schema.public

  column "experiment_id" {
    null = false
    type = uuid
  }
  # 1 gain, 0 loss
  column "side" {
    null = false
    type = smallint
  }
  # 1 quote, 0 percent
  column "unit" {
    null = false
    type = smallint
  }
  column "average" {
    null = false
    type = real
  }
  column "p10" {
    null = false
    type = real
  }
  column "p25" {
    null = false
    type = real
  }
  column "p50" {
    null = false
    type = real
  }
  column "p75" {
    null = false
    type = real
  }
  column "p90" {
    null = false
    type = real
  }
  column "p95" {
    null = false
    type = real
  }

  primary_key {
    columns = [column.experiment_id, column.side, column.unit]
  }
  foreign_key "experiment_outcome_stat_experiment_id_fk" {
    columns     = [column.experiment_id]
    ref_columns = [table.experiment.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
}

# One row per winning or losing sell. Histogram input for the instance and the experiment.
table "experiment_trade_outcome" {
  schema = schema.public

  column "instance_id" {
    null = false
    type = uuid
  }
  column "ordinal" {
    null = false
    type = integer
  }
  # 1 gain, 0 loss
  column "side" {
    null = false
    type = smallint
  }
  column "quote" {
    null = false
    type = real
  }
  column "percent" {
    null = true
    type = real
  }

  primary_key {
    columns = [column.instance_id, column.ordinal]
  }
  foreign_key "experiment_trade_outcome_instance_id_fk" {
    columns     = [column.instance_id]
    ref_columns = [table.experiment_instance.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
}
