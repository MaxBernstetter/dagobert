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

variable "allow_cloud_deploy" {
  description = "Must be true to touch real AWS (environment=prod). Set only by CI (TF_VAR_allow_cloud_deploy); never by hand."
  type        = bool
  default     = false
}

variable "vpc_cidr" {
  type    = string
  default = "10.0.0.0/16"
}

variable "database_name" {
  type    = string
  default = "trading"
}

variable "database_master_username" {
  type    = string
  default = "trading"
}

variable "database_master_password" {
  description = "Empty = generate one. Local sets a fixed value so Compose services can use it."
  type        = string
  default     = ""
  sensitive   = true
}

variable "database_port" {
  description = "Local pins a Floci proxy port (7001-7099) so Compose can address it."
  type        = number
  default     = 5432
}

variable "database_engine_version" {
  type    = string
  default = "16.4"
}

variable "database_min_acu" {
  type    = number
  default = 0.5
}

variable "database_max_acu" {
  type    = number
  default = 2
}

locals {
  is_local = var.environment == "local"
  prefix   = var.name_prefix != "" ? var.name_prefix : "${var.project}-${var.environment}"
  tags = {
    Project     = var.project
    Environment = var.environment
    ManagedBy   = "terraform"
    IdleCost    = "near-zero"
  }
}

variable "api_image_tag" {
  description = "Tag of the experiment-api image in ECR. scripts/infra-apply.sh builds and pushes it before the function is created."
  type        = string
  default     = "latest"
}

variable "api_cors_allow_origins" {
  description = "CORS origins the API Gateway accepts. No auth yet, so the default is open."
  type        = list(string)
  default     = ["*"]
}
