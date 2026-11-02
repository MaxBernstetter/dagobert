variable "environment" {
  description = "local (Floci) or prod (real AWS). Same modules; provider endpoints differ."
  type        = string
  default     = "local"

  validation {
    condition     = contains(["local", "prod"], var.environment)
    error_message = "environment must be local or prod."
  }
}

variable "aws_region" {
  type    = string
  default = "eu-central-1"
}

variable "project" {
  type    = string
  default = "dagobert"
}

variable "floci_endpoint" {
  description = "Floci / LocalStack base URL when environment=local"
  type        = string
  default     = "http://localhost:11566"
}

variable "name_prefix" {
  description = "Optional override for resource name prefix"
  type        = string
  default     = ""
}

variable "experiment_database_url" {
  description = "Postgres connection string for experiment-worker (catalog). Empty until set at apply time. Experiment catalog does not use DynamoDB."
  type        = string
  default     = ""
  sensitive   = true
}

locals {
  prefix = var.name_prefix != "" ? var.name_prefix : "${var.project}-${var.environment}"
  tags = {
    Project     = var.project
    Environment = var.environment
    ManagedBy   = "terraform"
    IdleCost    = "near-zero"
  }
}
