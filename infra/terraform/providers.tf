# One switch: var.environment. Credentials are the same on both: the AWS environment variables
# (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_REGION), from .env locally and the CI environment in prod.
#   local: every service endpoint on Floci. The skip_* flags below are Floci compatibility only.
#   prod:  real endpoints. Blocked unless var.allow_cloud_deploy (see gate.tf).

provider "aws" {
  region = var.aws_region

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
      ecs            = var.floci_endpoint
      apigateway     = var.floci_endpoint
      apigatewayv2   = var.floci_endpoint
      cloudfront     = var.floci_endpoint
      cloudwatch     = var.floci_endpoint
      cloudwatchlogs = var.floci_endpoint
      ec2            = var.floci_endpoint
      rds            = var.floci_endpoint
    }
  }

  default_tags {
    tags = local.tags
  }
}
