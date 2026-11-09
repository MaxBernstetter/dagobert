# One switch: var.environment.
#   local: static fake keys + every service endpoint on Floci. Nothing here can reach AWS.
#   prod:  default credential chain, real endpoints. Blocked unless var.allow_cloud_deploy (see gate.tf).

provider "aws" {
  region = var.aws_region

  # Floci accepts any non-empty keys; prod ignores these when using a profile/role.
  access_key = var.environment == "local" ? "test" : null
  secret_key = var.environment == "local" ? "test" : null

  skip_credentials_validation = var.environment == "local"
  skip_metadata_api_check     = var.environment == "local"
  skip_requesting_account_id  = var.environment == "local"
  s3_use_path_style           = var.environment == "local"

  dynamic "endpoints" {
    for_each = var.environment == "local" ? [1] : []
    content {
      s3             = var.floci_endpoint
      iam            = var.floci_endpoint
      sts            = var.floci_endpoint
      lambda         = var.floci_endpoint
      sfn            = var.floci_endpoint
      events         = var.floci_endpoint
      scheduler      = var.floci_endpoint
      ssm            = var.floci_endpoint
      ecr            = var.floci_endpoint
      apigateway     = var.floci_endpoint
      apigatewayv2   = var.floci_endpoint
      cloudwatch     = var.floci_endpoint
      cloudwatchlogs = var.floci_endpoint
      ecs            = var.floci_endpoint
      ec2            = var.floci_endpoint
      rds            = var.floci_endpoint
    }
  }

  default_tags {
    tags = local.tags
  }
}
