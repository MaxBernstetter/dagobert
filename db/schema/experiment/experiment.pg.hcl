table "experiment" {
  schema = schema.public

  column "id" {
    null = false
    type = uuid
  }
  column "name" {
    null = false
    type = text
  }
  column "description" {
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

  # Typed experiment settings (no jsonb documents).
  column "provider" {
    null = false
    type = text
    default = "binance"
  }
  column "base" {
    null = false
    type = text
    default = "BTC"
  }
  column "quote" {
    null = false
    type = text
    default = "USDT"
  }
  column "from_ms" {
    null = false
    type = bigint
    default = 0
  }
  column "to_ms" {
    null = false
    type = bigint
    default = 0
  }
  column "initial_usdc" {
    null = false
    type = double_precision
    default = 0
  }
  column "initial_usdt" {
    null = false
    type = double_precision
    default = 0
  }
  column "initial_base" {
    null = false
    type = double_precision
    default = 0
  }
  column "target_weight_pct" {
    null = false
    type = double_precision
    default = 0
  }
  column "cash_reserve_quote" {
    null = false
    type = double_precision
    default = 0
  }
  column "min_order_quote" {
    null = false
    type = double_precision
    default = 0
  }
  column "redownload_missing" {
    null = false
    type = boolean
    default = false
  }
  column "algo" {
    null = false
    type = text
    default = "v1"
  }
  column "spread_bps" {
    null = false
    type = double_precision
    default = 0
  }
  column "slippage_bps" {
    null = false
    type = double_precision
    default = 0
  }
  column "fee_bps" {
    null = false
    type = double_precision
    default = 0
  }
  # C++ sweep processes of the run. NULL (or 0) = auto: one per CPU of the worker.
  column "parallelism" {
    null = true
    type = integer
  }

  column "created_at_ms" {
    null = false
    type = bigint
  }
  column "started_at_ms" {
    null = true
    type = bigint
  }
  column "finished_at_ms" {
    null = true
    type = bigint
  }

  column "progress_stage" {
    null = false
    type = integer
    default = 0
  }
  column "progress_stages" {
    null = false
    type = integer
    default = 0
  }
  column "progress_stage_label" {
    null = false
    type = text
    default = ""
  }
  column "progress_instance_done" {
    null = false
    type = integer
    default = 0
  }
  column "progress_instance_total" {
    null = false
    type = integer
    default = 0
  }

  primary_key {
    columns = [column.id]
  }
  index "experiment_created_at_ms_idx" {
    columns = [column.created_at_ms]
  }
  index "experiment_status_idx" {
    columns = [column.status]
  }
}
