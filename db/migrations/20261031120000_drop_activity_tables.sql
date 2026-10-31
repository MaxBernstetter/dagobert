-- Create "experiment_instance_fill" table dropped: fills live in each instance's fills.parquet.
-- Fills reference actions, so they go first.
DROP TABLE "experiment_instance_fill";
-- Create "experiment_instance_action" table dropped: unused, the order and action rows live in orders.parquet.
DROP TABLE "experiment_instance_action";
-- Create "experiment_trade_outcome" table dropped: trade outcomes live in each instance's trade_outcomes.parquet.
DROP TABLE "experiment_trade_outcome";
