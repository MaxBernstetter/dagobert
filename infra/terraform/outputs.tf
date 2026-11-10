output "environment" {
  value = var.environment
}

output "lake_bucket" {
  value = aws_s3_bucket.lake.bucket
}

output "task_role_arn" {
  value = aws_iam_role.task.arn
}

output "ssm_prefix" {
  value = "/${var.project}/${var.environment}"
}

output "database_host" {
  value = aws_rds_cluster.main.endpoint
}

output "database_port" {
  value = aws_rds_cluster.main.port
}

output "database_url_ssm" {
  value = aws_ssm_parameter.database_url.name
}

output "floci_hint" {
  value = var.environment == "local" ? "export AWS_ENDPOINT_URL=${var.floci_endpoint} && eval $(floci env) 2>/dev/null || true" : "unset AWS_ENDPOINT_URL; use real AWS credentials"
}

output "experiment_sfn_arn" {
  value = aws_sfn_state_machine.experiment_run.arn
}

output "experiment_worker_arn" {
  value = aws_lambda_function.experiment_worker.arn
}

output "experiment_worker_repository_url" {
  value = aws_ecr_repository.experiment_worker.repository_url
}

output "frontend_bucket" {
  value = aws_s3_bucket.frontend.bucket
}

output "frontend_distribution_id" {
  value = aws_cloudfront_distribution.frontend.id
}

output "frontend_domain" {
  description = "CloudFront domain. Local: send it as the Host header to the Floci port (see docs/architecture/local-stack.md)."
  value       = aws_cloudfront_distribution.frontend.domain_name
}

output "api_gateway_url" {
  value = aws_apigatewayv2_api.experiment.api_endpoint
}

output "experiment_api_repository_url" {
  value = aws_ecr_repository.experiment_api.repository_url
}

output "backfill_sfn_arn" {
  description = "Step Functions state machine: ingest then indicator for one market. scripts/backfill-start.sh starts it."
  value       = aws_sfn_state_machine.backfill.arn
}

output "jobs_cluster" {
  value = aws_ecs_cluster.jobs.name
}

output "ingest_repository_url" {
  value = aws_ecr_repository.ingest.repository_url
}

output "indicator_repository_url" {
  value = aws_ecr_repository.indicator.repository_url
}

# scripts/db-migrate.sh: the migration task's cluster, task family, network and log group.
output "db_migrate_repository_url" {
  value = aws_ecr_repository.db_migrate.repository_url
}

output "db_migrate_cluster" {
  value = aws_ecs_cluster.ops.name
}

output "db_migrate_task_family" {
  value = aws_ecs_task_definition.db_migrate.family
}

output "db_migrate_subnet_ids" {
  value = join(",", aws_subnet.private[*].id)
}

output "db_migrate_security_group_id" {
  value = aws_security_group.app.id
}

output "db_migrate_log_group" {
  value = aws_cloudwatch_log_group.db_migrate.name
}
