# The market snapshot written by a successful provider setup. ordinal keeps the upstream order.
table "provider_pair" {
  schema = schema.public

  column "provider_id" {
    null = false
    type = text
  }
  column "ordinal" {
    null = false
    type = integer
  }
  column "base" {
    null = false
    type = text
  }
  column "quote" {
    null = false
    type = text
  }

  primary_key {
    columns = [column.provider_id, column.base, column.quote]
  }
  foreign_key "provider_pair_provider_id_fk" {
    columns     = [column.provider_id]
    ref_columns = [table.provider.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
  index "provider_pair_provider_ordinal_idx" {
    columns = [column.provider_id, column.ordinal]
  }
}
