-- Modify "experiment" table: the C++ sweep process count chosen for the experiment. NULL = auto (one per CPU).
ALTER TABLE "experiment" ADD COLUMN "parallelism" integer NULL;
