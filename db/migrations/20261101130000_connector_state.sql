-- Create "provider" table: ingestion provider activation, setup status and progress, non-secret config.
-- Secrets stay in the file-based secrets store.
CREATE TABLE "provider" (
  "id" text NOT NULL,
  "activated" boolean NOT NULL DEFAULT false,
  "status" text NOT NULL DEFAULT 'inactive',
  "error" text NOT NULL DEFAULT '',
  "progress_label" text NULL,
  "progress_done" integer NULL,
  "progress_total" integer NULL,
  "config" jsonb NULL,
  "updated_at_ms" bigint NOT NULL DEFAULT 0,
  PRIMARY KEY ("id")
);
-- Create "provider_pair" table: the market snapshot of a successful provider setup.
CREATE TABLE "provider_pair" (
  "provider_id" text NOT NULL,
  "ordinal" integer NOT NULL,
  "base" text NOT NULL,
  "quote" text NOT NULL,
  PRIMARY KEY ("provider_id", "base", "quote"),
  CONSTRAINT "provider_pair_provider_id_fk" FOREIGN KEY ("provider_id") REFERENCES "provider" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
CREATE INDEX "provider_pair_provider_ordinal_idx" ON "provider_pair" ("provider_id", "ordinal");
-- Supabase exposes new public tables to the API roles by default. Revoke them where those roles exist
-- (a plain Postgres has none, so the migration still applies there).
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
    REVOKE ALL ON TABLE "provider", "provider_pair" FROM anon, authenticated;
  END IF;
END $$;
