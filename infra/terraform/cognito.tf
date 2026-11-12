# Sign-in for the web app: a Cognito user pool with one public app client. The API Lambda signs users in through it
# (POST /auth/login) and accepts only calls that carry a valid access token (src/internal/auth). Floci does not
# validate JWTs at API Gateway and has no CORS on its Cognito endpoint, so both targets use the Lambda.
#   local (Floci): the pool allows 6-character passwords with no other rule, and Terraform seeds the user
#                  admin / password so the stack is usable after `task up`.
#   prod:          the default strong policy and no seeded user. Create users with
#                  `aws cognito-idp admin-create-user` (self sign-up is off).

variable "cognito_local_username" {
  description = "Local only: the user that Terraform creates in the Floci user pool."
  type        = string
  default     = "admin"
}

variable "cognito_local_password" {
  description = "Local only: its password. Cognito requires at least 6 characters, so `admin` is not possible."
  type        = string
  default     = "password"
  sensitive   = true
}

resource "aws_cognito_user_pool" "main" {
  name = "${local.prefix}-users"

  admin_create_user_config {
    allow_admin_create_user_only = true
  }

  password_policy {
    minimum_length                   = local.is_local ? 6 : 12
    require_lowercase                = !local.is_local
    require_uppercase                = !local.is_local
    require_numbers                  = !local.is_local
    require_symbols                  = !local.is_local
    temporary_password_validity_days = 7
  }

  depends_on = [terraform_data.deploy_gate]
}

resource "aws_cognito_user_pool_client" "web" {
  name         = "${local.prefix}-web"
  user_pool_id = aws_cognito_user_pool.main.id

  generate_secret = false
  explicit_auth_flows = [
    "ALLOW_USER_PASSWORD_AUTH",
    "ALLOW_REFRESH_TOKEN_AUTH",
  ]
  prevent_user_existence_errors = "ENABLED"
  access_token_validity         = 60
  id_token_validity             = 60
  refresh_token_validity        = 30
  token_validity_units {
    access_token  = "minutes"
    id_token      = "minutes"
    refresh_token = "days"
  }
}

resource "aws_cognito_user" "local" {
  count        = local.is_local ? 1 : 0
  user_pool_id = aws_cognito_user_pool.main.id
  username     = var.cognito_local_username
  password     = var.cognito_local_password
}
