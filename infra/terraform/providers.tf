# Local (Floci): point every used service at :4566.
# Prod: omit custom endpoints; use real IAM / SSO credentials.

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
    }
  }

  default_tags {
    tags = local.tags
  }
}

# Token comes from .env SUPABASE_ACCESS_TOKEN. Task maps it to TF_VAR_supabase_access_token.
# Leave the variable empty on local applies so this provider is not called.
provider "supabase" {
  access_token = var.supabase_access_token
}
