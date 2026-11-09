# Relational control plane: Aurora PostgreSQL Serverless v2 (experiments, stats, activity).
# Schema is owned by Atlas (db/migrations). The URL lands in SSM for the services that connect.

resource "random_password" "database" {
  length  = 32
  special = false
}

locals {
  database_password = var.database_master_password != "" ? var.database_master_password : random_password.database.result
  database_url = format(
    "postgres://%s:%s@%s:%d/%s?sslmode=require&search_path=public",
    var.database_master_username,
    urlencode(local.database_password),
    aws_rds_cluster.main.endpoint,
    aws_rds_cluster.main.port,
    var.database_name,
  )
}

resource "aws_rds_cluster" "main" {
  cluster_identifier     = "${local.prefix}-db"
  engine                 = "aurora-postgresql"
  engine_version         = var.database_engine_version
  database_name          = var.database_name
  port                   = var.database_port
  master_username        = var.database_master_username
  master_password        = local.database_password
  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.database.id]
  storage_encrypted      = true
  deletion_protection    = var.environment == "prod"
  skip_final_snapshot    = var.environment == "local"

  serverlessv2_scaling_configuration {
    min_capacity = var.database_min_acu
    max_capacity = var.database_max_acu
  }

  lifecycle {
    ignore_changes = [master_password]
  }
}

resource "aws_rds_cluster_instance" "writer" {
  identifier         = "${local.prefix}-db-writer"
  cluster_identifier = aws_rds_cluster.main.id
  engine             = aws_rds_cluster.main.engine
  engine_version     = aws_rds_cluster.main.engine_version
  instance_class     = "db.serverless"
}

resource "aws_ssm_parameter" "database_url" {
  name  = "/${var.project}/${var.environment}/DATABASE_URL"
  type  = "SecureString"
  value = local.database_url
}
