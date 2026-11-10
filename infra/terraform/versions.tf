terraform {
  required_version = ">= 1.10.0" # use_lockfile (S3 native locking) needs 1.10

  # The state bucket is created by infra/bootstrap; scripts/lib/terraform.sh passes the bucket and key at init.
  backend "s3" {}

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    archive = {
      source  = "hashicorp/archive"
      version = "~> 2.4"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }
}
