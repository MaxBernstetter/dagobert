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

output "supabase_project_id" {
  value = local.create_supabase ? supabase_project.control[0].id : ""
}

output "database_url_ssm" {
  value = local.create_supabase ? aws_ssm_parameter.database_url[0].name : ""
}

output "floci_hint" {
  value = var.environment == "local" ? "export AWS_ENDPOINT_URL=${var.floci_endpoint} && eval $(floci env) 2>/dev/null || true" : "unset AWS_ENDPOINT_URL; use real AWS credentials"
}

output "experiment_sfn_arn" {
  value = aws_sfn_state_machine.experiment_run.arn
}

output "experiment_worker_arn" {
  value = var.enable_experiment_lambda ? aws_lambda_function.experiment_worker[0].arn : ""
}
