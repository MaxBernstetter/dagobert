# Hours the exchange has no candle for (confirmed empty). Coverage checks treat only retryable gaps as blocking.
table "ingest_unavailable" {
  schema = schema.public

  column "provider" {
    null = false
    type = text
  }
  column "base" {
    null = false
    type = text
  }
  column "quote" {
    null = false
    type = text
  }
  column "open_time" {
    null = false
    type = bigint
  }
  column "reason" {
    null    = false
    type    = text
    default = ""
  }
  column "confirmed_at" {
    null = false
    type = bigint
  }

  primary_key {
    columns = [column.provider, column.base, column.quote, column.open_time]
  }
}
