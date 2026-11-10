# One-off database migrations as an ECS Fargate task inside the VPC (reaches Aurora, which is private).
# No service, no schedule: scripts/db-migrate.sh builds the image, runs the task with aws ecs run-task,
# waits for it to stop and reads the container exit code. The task runs the image in docker/db-migrate.
# No public IP. Image pulls need the ECR, S3 and Logs paths: the S3 gateway endpoint (network.tf) plus,
# in prod only, the interface endpoints below. Floci needs none.

resource "aws_ecr_repository" "db_migrate" {
  name                 = "${local.prefix}-db-migrate"
  image_tag_mutability = "MUTABLE"
  force_delete         = local.is_local

  depends_on = [terraform_data.deploy_gate]
}

resource "aws_ecs_cluster" "ops" {
  name = "${local.prefix}-ops"
}

resource "aws_cloudwatch_log_group" "db_migrate" {
  name              = "/ecs/${local.prefix}-db-migrate"
  retention_in_days = 14
}

# ecs_tasks_assume (the ECS tasks trust policy) is defined in jobs_ecs.tf.

# Pulls the image, writes the logs and reads the DATABASE_URL secret for the container.
resource "aws_iam_role" "db_migrate_execution" {
  name               = "${local.prefix}-db-migrate-execution"
  assume_role_policy = data.aws_iam_policy_document.ecs_tasks_assume.json
}

resource "aws_iam_role_policy_attachment" "db_migrate_execution" {
  role       = aws_iam_role.db_migrate_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

data "aws_iam_policy_document" "db_migrate_execution_secret" {
  statement {
    sid       = "DatabaseUrl"
    actions   = ["ssm:GetParameters"]
    resources = [aws_ssm_parameter.database_url.arn]
  }
}

resource "aws_iam_role_policy" "db_migrate_execution_secret" {
  name   = "${local.prefix}-db-migrate-secret"
  role   = aws_iam_role.db_migrate_execution.id
  policy = data.aws_iam_policy_document.db_migrate_execution_secret.json
}

# The container makes no AWS API calls: an empty task role.
resource "aws_iam_role" "db_migrate_task" {
  name               = "${local.prefix}-db-migrate-task"
  assume_role_policy = data.aws_iam_policy_document.ecs_tasks_assume.json
}

# Migrations run with the app security group, so the existing database rule (app -> 5432) applies.
resource "aws_ecs_task_definition" "db_migrate" {
  family                   = "${local.prefix}-db-migrate"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = aws_iam_role.db_migrate_execution.arn
  task_role_arn            = aws_iam_role.db_migrate_task.arn

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "X86_64"
  }

  container_definitions = jsonencode([
    {
      name      = "db-migrate"
      image     = "${aws_ecr_repository.db_migrate.repository_url}:latest"
      essential = true
      secrets = [
        { name = "DATABASE_URL", valueFrom = aws_ssm_parameter.database_url.arn },
      ]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = aws_cloudwatch_log_group.db_migrate.name
          awslogs-region        = var.aws_region
          awslogs-stream-prefix = "migrate"
        }
      }
    },
  ])
}

# Interface endpoints for private Fargate image pulls, logs and secrets, and for the API Lambda's provider
# secrets (secretsmanager). Prod only (no NAT, no internet).
# One subnet each keeps the idle cost down; the task can run in any private subnet and reach the endpoint ENI.
locals {
  db_migrate_interface_endpoints = var.environment == "prod" ? toset(["ecr.api", "ecr.dkr", "logs", "ssm", "secretsmanager"]) : toset([])
}

resource "aws_security_group" "vpc_endpoints" {
  name        = "${local.prefix}-vpc-endpoints"
  description = "Interface VPC endpoints: HTTPS from the app security group"
  vpc_id      = aws_vpc.main.id
}

resource "aws_vpc_security_group_ingress_rule" "vpc_endpoints_from_app" {
  security_group_id            = aws_security_group.vpc_endpoints.id
  referenced_security_group_id = aws_security_group.app.id
  ip_protocol                  = "tcp"
  from_port                    = 443
  to_port                      = 443
}

resource "aws_vpc_endpoint" "interface" {
  for_each            = local.db_migrate_interface_endpoints
  vpc_id              = aws_vpc.main.id
  service_name        = "com.amazonaws.${var.aws_region}.${each.key}"
  vpc_endpoint_type   = "Interface"
  subnet_ids          = [aws_subnet.private[0].id]
  security_group_ids  = [aws_security_group.vpc_endpoints.id]
  private_dns_enabled = true
}
