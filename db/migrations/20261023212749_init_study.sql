-- Create "study" table
CREATE TABLE "study" (
  "id" uuid NOT NULL,
  "name" text NOT NULL,
  "status" text NOT NULL,
  "created_at_ms" bigint NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "study_created_at_ms_idx" to table: "study"
CREATE INDEX "study_created_at_ms_idx" ON "study" ("created_at_ms");
-- Create "study_instance" table
CREATE TABLE "study_instance" (
  "id" uuid NOT NULL,
  "study_id" uuid NOT NULL,
  "playbook" text NOT NULL,
  "symbol" text NOT NULL,
  "status" text NOT NULL,
  "metric_sharpe" double precision NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "study_instance_study_id_fk" FOREIGN KEY ("study_id") REFERENCES "study" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "study_instance_study_id_sharpe_idx" to table: "study_instance"
CREATE INDEX "study_instance_study_id_sharpe_idx" ON "study_instance" ("study_id", "metric_sharpe");
