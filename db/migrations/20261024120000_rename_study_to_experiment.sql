-- Rename study control-plane tables to experiment (historical migrations keep study names).
ALTER TABLE "study" RENAME TO "experiment";
ALTER TABLE "study_instance" RENAME TO "experiment_instance";
ALTER TABLE "study_instance_action" RENAME TO "experiment_instance_action";
ALTER TABLE "study_instance_fill" RENAME TO "experiment_instance_fill";
ALTER TABLE "study_instance_period_stat" RENAME TO "experiment_instance_period_stat";

ALTER TABLE "experiment_instance" RENAME COLUMN "study_id" TO "experiment_id";

ALTER INDEX "study_created_at_ms_idx" RENAME TO "experiment_created_at_ms_idx";
ALTER INDEX "study_status_idx" RENAME TO "experiment_status_idx";
ALTER INDEX "study_instance_study_id_idx" RENAME TO "experiment_instance_experiment_id_idx";
ALTER INDEX "study_instance_study_id_status_idx" RENAME TO "experiment_instance_experiment_id_status_idx";
ALTER INDEX "study_instance_study_id_symbol_idx" RENAME TO "experiment_instance_experiment_id_symbol_idx";
ALTER INDEX "study_instance_study_id_sharpe_idx" RENAME TO "experiment_instance_experiment_id_sharpe_idx";
ALTER INDEX "study_instance_study_id_sortino_idx" RENAME TO "experiment_instance_experiment_id_sortino_idx";
ALTER INDEX "study_instance_study_id_cagr_idx" RENAME TO "experiment_instance_experiment_id_cagr_idx";
ALTER INDEX "study_instance_study_id_maxdd_idx" RENAME TO "experiment_instance_experiment_id_maxdd_idx";
ALTER INDEX "study_instance_study_id_duration_idx" RENAME TO "experiment_instance_experiment_id_duration_idx";
ALTER INDEX "study_instance_study_id_mo_ret_idx" RENAME TO "experiment_instance_experiment_id_mo_ret_idx";

ALTER TABLE "experiment_instance" RENAME CONSTRAINT "study_instance_study_id_fk" TO "experiment_instance_experiment_id_fk";

ALTER TABLE "experiment_instance_action" RENAME COLUMN "study_id" TO "experiment_id";
ALTER INDEX "study_instance_action_study_id_idx" RENAME TO "experiment_instance_action_experiment_id_idx";
ALTER TABLE "experiment_instance_action" RENAME CONSTRAINT "study_instance_action_study_id_fk" TO "experiment_instance_action_experiment_id_fk";

ALTER TABLE "experiment_instance_fill" RENAME COLUMN "study_id" TO "experiment_id";
ALTER TABLE "experiment_instance_fill" RENAME CONSTRAINT "study_instance_fill_study_id_fk" TO "experiment_instance_fill_experiment_id_fk";

ALTER TABLE "experiment_instance_period_stat" RENAME CONSTRAINT "study_instance_period_stat_instance_id_fk" TO "experiment_instance_period_stat_instance_id_fk";
