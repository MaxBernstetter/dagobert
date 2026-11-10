output "state_bucket" {
  description = "Bucket for the infra/terraform state (scripts/lib/terraform.sh passes it to init)."
  value       = aws_s3_bucket.state.bucket
}
