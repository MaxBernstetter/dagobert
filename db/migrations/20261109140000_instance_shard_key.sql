-- Add "shard_key" to "experiment_instance": the data-dir-relative directory of the shard that holds
-- the instance's Parquet row groups. Empty until the instance completes. Metadata, not instance data.
ALTER TABLE "experiment_instance" ADD COLUMN "shard_key" text NOT NULL DEFAULT '';
