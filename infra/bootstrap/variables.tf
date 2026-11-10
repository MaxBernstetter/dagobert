variable "environment" {
  description = "local (Floci) or prod (real AWS). Same switch as infra/terraform."
  type        = string
  default     = "local"

  validation {
    condition     = contains(["local", "prod"], var.environment)
    error_message = "environment must be local or prod."
  }
}

variable "aws_region" {
  type = string
}

variable "project" {
  type    = string
  default = "dagobert"
}

variable "floci_endpoint" {
  description = "Floci base URL when environment=local"
  type        = string
  default     = "http://localhost:11566"
}

variable "allow_cloud_deploy" {
  description = "Must be true to touch real AWS. Set only by CI (TF_VAR_allow_cloud_deploy)."
  type        = bool
  default     = false
}
