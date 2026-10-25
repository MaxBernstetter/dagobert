-- Modify "study" table
ALTER TABLE "study" ALTER COLUMN "from_ms" SET DEFAULT 0, ALTER COLUMN "to_ms" SET DEFAULT 0;
-- Modify "study_instance" table
ALTER TABLE "study_instance" ALTER COLUMN "config_hash" SET DEFAULT '', ALTER COLUMN "playbook_key" SET DEFAULT '', ALTER COLUMN "workspace_rel_path" SET DEFAULT '';
