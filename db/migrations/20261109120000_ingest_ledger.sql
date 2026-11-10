-- Create "ingest_stored" table: hours whose candles are stored in the lake (was ingest.sqlite "stored").
CREATE TABLE "ingest_stored" (
  "provider" text NOT NULL,
  "base" text NOT NULL,
  "quote" text NOT NULL,
  "open_time" bigint NOT NULL,
  PRIMARY KEY ("provider", "base", "quote", "open_time")
);
-- Create "ingest_unavailable" table: hours the exchange has no candle for (was ingest.sqlite "unavailable").
CREATE TABLE "ingest_unavailable" (
  "provider" text NOT NULL,
  "base" text NOT NULL,
  "quote" text NOT NULL,
  "open_time" bigint NOT NULL,
  "reason" text NOT NULL DEFAULT '',
  "confirmed_at" bigint NOT NULL,
  PRIMARY KEY ("provider", "base", "quote", "open_time")
);
