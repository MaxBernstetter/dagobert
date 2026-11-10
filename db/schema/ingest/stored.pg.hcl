# Ingest ledger: the hours whose candles are stored in the lake. Replaces ingest.sqlite.
table "ingest_stored" {
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

  primary_key {
    columns = [column.provider, column.base, column.quote, column.open_time]
  }
}
