# Data-prep jobs as ECS Fargate tasks: ingest (Binance klines into the lake) and the technical-indicator backfill.
# The orchestrator no longer starts them through the Docker socket for backfills; backfill_sfn.tf runs them in order.
# Same images and exit-code contract as the containers (0 succeeded, 1 failed, 2 aborted = nothing to do).
# scripts/infra-apply.sh creates the repositories first, publishes the images, then the task definitions reference them.

locals {
  jobs_data_dir = "/tmp/dagobert-data" # scratch only; the lake lives in S3 (LAKE_BACKEND=s3)
  jobs_env = [
    { name = "DATA_DIR", value = local.jobs_data_dir },
    { name = "LAKE_BUCKET", value = aws_s3_bucket.lake.bucket },
    { name = "LAKE_BACKEND", value = "s3" },
    { name = "AWS_REGION", value = var.aws_region },
    { name = "DEPLOY_ENV", value = var.environment },
    { name = "SSM_PREFIX", value = "/${var.project}/${var.environment}" },
    # Exit 2 = already stored, counts as done: the container exits 0 so runTask.sync continues to the next state.
    { name = "COMPOSE_OK_ON_ABORT", value = "1" },
  ]
}

resource "aws_ecr_repository" "ingest" {
  name                 = "${local.prefix}-ingest"
  image_tag_mutability = "MUTABLE"
  force_delete         = var.environment == "local"

  depends_on = [terraform_data.deploy_gate]
}

resource "aws_ecr_repository" "indicator" {
  name                 = "${local.prefix}-indicator"
  image_tag_mutability = "MUTABLE"
  force_delete         = var.environment == "local"

  depends_on = [terraform_data.deploy_gate]
}

resource "aws_cloudwatch_log_group" "ingest" {
  name              = "/ecs/${local.prefix}-ingest"
  retention_in_days = 14
}

resource "aws_cloudwatch_log_group" "indicator" {
  name              = "/ecs/${local.prefix}-indicator"
  retention_in_days = 14
}

resource "aws_ecs_cluster" "jobs" {
  name = "${local.prefix}-jobs"
}

data "aws_iam_policy_document" "ecs_tasks_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

# Execution role: pull the images, write logs, and read the SSM parameters injected as task secrets.
# The task role is the shared aws_iam_role.task (main.tf): S3 lake read/write and SSM read.
resource "aws_iam_role" "jobs_execution" {
  name               = "${local.prefix}-jobs-execution"
  assume_role_policy = data.aws_iam_policy_document.ecs_tasks_assume.json
}

data "aws_iam_policy_document" "jobs_execution" {
  statement {
    sid       = "EcrAuth"
    actions   = ["ecr:GetAuthorizationToken"]
    resources = ["*"]
  }

  statement {
    sid = "EcrPull"
    actions = [
      "ecr:BatchGetImage",
      "ecr:GetDownloadUrlForLayer",
      "ecr:BatchCheckLayerAvailability",
    ]
    resources = [aws_ecr_repository.ingest.arn, aws_ecr_repository.indicator.arn]
  }

  statement {
    sid = "Logs"
    actions = [
      "logs:CreateLogStream",
      "logs:PutLogEvents",
    ]
    resources = [
      "${aws_cloudwatch_log_group.ingest.arn}:*",
      "${aws_cloudwatch_log_group.indicator.arn}:*",
    ]
  }

  # Secrets are injected by the ECS agent with the execution role, not the task role.
  statement {
    sid     = "SecretsFromSSM"
    actions = ["ssm:GetParameters"]
    resources = [
      aws_ssm_parameter.database_url.arn,
      aws_ssm_parameter.binance_base_url.arn,
    ]
  }
}

resource "aws_iam_role_policy" "jobs_execution" {
  name   = "${local.prefix}-jobs-execution"
  role   = aws_iam_role.jobs_execution.id
  policy = data.aws_iam_policy_document.jobs_execution.json
}

# Tasks: public subnets (exchange and public AWS endpoints), no ingress. Egress 443 anywhere, and Aurora on 5432.
resource "aws_security_group" "jobs" {
  name        = "${local.prefix}-jobs"
  description = "ECS data-prep tasks: HTTPS out (exchange, AWS APIs), PostgreSQL to Aurora. No ingress."
  vpc_id      = aws_vpc.main.id

  egress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 5432
    to_port     = 5432
    protocol    = "tcp"
    cidr_blocks = [var.vpc_cidr]
  }
}

resource "aws_ecs_task_definition" "ingest" {
  family                   = "${local.prefix}-ingest"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 512
  memory                   = 1024
  execution_role_arn       = aws_iam_role.jobs_execution.arn
  task_role_arn            = aws_iam_role.task.arn

  # Args come from the state machine (backfill_sfn.tf): -from -to -base -quote -provider.
  container_definitions = jsonencode([{
    name        = "ingest"
    image       = "${aws_ecr_repository.ingest.repository_url}:${var.job_image_tag}"
    essential   = true
    environment = local.jobs_env
    # BINANCE_BASE_URL is not a secret: backfill_sfn.tf passes it per execution (container override).
    secrets = [
      { name = "DATABASE_URL", valueFrom = aws_ssm_parameter.database_url.arn },
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.ingest.name
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "ingest"
      }
    }
  }])
}

resource "aws_ecs_task_definition" "indicator" {
  family                   = "${local.prefix}-indicator"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 1024
  memory                   = 2048
  execution_role_arn       = aws_iam_role.jobs_execution.arn
  task_role_arn            = aws_iam_role.task.arn

  # Args come from the state machine (backfill_sfn.tf): symbol from to. No connector secrets needed.
  container_definitions = jsonencode([{
    name        = "indicator"
    image       = "${aws_ecr_repository.indicator.repository_url}:${var.job_image_tag}"
    essential   = true
    environment = local.jobs_env
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.indicator.name
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "indicator"
      }
    }
  }])
}
