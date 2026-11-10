# The state bucket for infra/terraform. Applied before it, with local state (infra/bootstrap/terraform.tfstate, gitignored).
# Same switch as the main root: local talks only to Floci, prod needs CI (allow_cloud_deploy).

resource "terraform_data" "deploy_gate" {
  lifecycle {
    precondition {
      condition     = var.environment == "local" || var.allow_cloud_deploy
      error_message = "Refusing to create the state bucket in real AWS: environment=prod requires allow_cloud_deploy=true (CI only)."
    }
  }
}

provider "aws" {
  region = var.aws_region

  skip_credentials_validation = var.environment == "local"
  skip_metadata_api_check     = var.environment == "local"
  skip_requesting_account_id  = var.environment == "local"
  s3_use_path_style           = var.environment == "local"

  dynamic "endpoints" {
    for_each = var.environment == "local" ? [1] : []
    content {
      s3  = var.floci_endpoint
      sts = var.floci_endpoint
    }
  }
}
