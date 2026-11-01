# One row per ingestion provider: activation, setup status and progress, non-secret config.
# Secrets stay in the file-based secrets store and are never written here.
table "provider" {
  schema = schema.public

  column "id" {
    null = false
    type = text
  }
  column "activated" {
    null = false
    type = boolean
    default = false
  }
  column "status" {
    null = false
    type = text
    default = "inactive"
  }
  column "error" {
    null = false
    type = text
    default = ""
  }
  # NULL: no progress reported. Set together with the label.
  column "progress_label" {
    null = true
    type = text
  }
  column "progress_done" {
    null = true
    type = integer
  }
  column "progress_total" {
    null = true
    type = integer
  }
  # NULL: the config was never saved (the view then shows the default base URL).
  column "config" {
    null = true
    type = jsonb
  }
  column "updated_at_ms" {
    null = false
    type = bigint
    default = 0
  }

  primary_key {
    columns = [column.id]
  }
}
