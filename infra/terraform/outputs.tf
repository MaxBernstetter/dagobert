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
