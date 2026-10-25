# Prod control plane: one Supabase Free project (public schema only).
# Local Floci does not create this. Schema changes stay in Atlas (db/migrations).

resource "supabase_project" "control" {
  count = local.create_supabase ? 1 : 0

  organization_id   = var.supabase_organization_id
  name              = local.prefix
  database_password = var.supabase_database_password
  region            = local.supabase_region

  lifecycle {
    precondition {
      condition     = var.supabase_access_token != "" && var.supabase_database_password != ""
      error_message = "supabase_access_token and supabase_database_password are required when supabase_organization_id is set."
    }
  }
}

locals {
  supabase_database_url = local.create_supabase ? format(
    "postgres://postgres:%s@db.%s.supabase.co:5432/postgres?sslmode=require&search_path=public",
    urlencode(var.supabase_database_password),
    supabase_project.control[0].id,
  ) : ""
}

resource "aws_ssm_parameter" "database_url" {
  count = local.create_supabase ? 1 : 0

  name  = "/${var.project}/${var.environment}/DATABASE_URL"
  type  = "SecureString"
  value = local.supabase_database_url

  lifecycle {
    ignore_changes = [value]
  }
}
