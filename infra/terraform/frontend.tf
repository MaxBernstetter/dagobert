# The web app: the production frontend image (Vite SPA) as an ECS Fargate service.
# scripts/infra-apply.sh creates the repository first, publishes the image, then creates the service.

# Empty frontend_api_url = the Experiment API gateway (api_lambda.tf). No cycle: the API does not depend on the frontend.
locals {
  frontend_api_url = var.frontend_api_url != "" ? var.frontend_api_url : aws_apigatewayv2_api.experiment.api_endpoint
}

resource "aws_ecr_repository" "frontend" {
  name                 = "${local.prefix}-frontend"
  image_tag_mutability = "MUTABLE"
  force_delete         = var.environment == "local"

  depends_on = [terraform_data.deploy_gate]
}

resource "aws_cloudwatch_log_group" "frontend" {
  name              = "/ecs/${local.prefix}-frontend"
  retention_in_days = 14
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

# Execution role: pull the image and write logs. The task itself needs no AWS access.
resource "aws_iam_role" "frontend_execution" {
  name               = "${local.prefix}-frontend-execution"
  assume_role_policy = data.aws_iam_policy_document.ecs_tasks_assume.json
}

data "aws_iam_policy_document" "frontend_execution" {
  statement {
    sid       = "EcrAuth"
    actions   = ["ecr:GetAuthorizationToken"]
    resources = ["*"]
  }
  statement {
    sid       = "EcrPull"
    actions   = ["ecr:BatchGetImage", "ecr:GetDownloadUrlForLayer", "ecr:BatchCheckLayerAvailability"]
    resources = [aws_ecr_repository.frontend.arn]
  }
  statement {
    sid       = "Logs"
    actions   = ["logs:CreateLogStream", "logs:PutLogEvents"]
    resources = ["${aws_cloudwatch_log_group.frontend.arn}:*"]
  }
}

resource "aws_iam_role_policy" "frontend_execution" {
  name   = "${local.prefix}-frontend-execution"
  role   = aws_iam_role.frontend_execution.id
  policy = data.aws_iam_policy_document.frontend_execution.json
}

resource "aws_security_group" "frontend" {
  name        = "${local.prefix}-frontend"
  description = "Web app: HTTP from anywhere, any egress"
  vpc_id      = aws_vpc.main.id

  ingress {
    from_port   = var.frontend_port
    to_port     = var.frontend_port
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_ecs_cluster" "main" {
  name = local.prefix
}

resource "aws_ecs_task_definition" "frontend" {
  family                   = "${local.prefix}-frontend"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = aws_iam_role.frontend_execution.arn

  container_definitions = jsonencode([{
    name      = "frontend"
    image     = "${aws_ecr_repository.frontend.repository_url}:${var.frontend_image_tag}"
    essential = true
    command   = ["npm", "run", "preview", "--", "--host", "0.0.0.0", "--port", tostring(var.frontend_port)]
    portMappings = [{
      containerPort = var.frontend_port
      hostPort      = var.frontend_port
      protocol      = "tcp"
    }]
    environment = [{ name = "VITE_DEV_API_PROXY", value = local.frontend_api_url }]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.frontend.name
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "frontend"
      }
    }
  }])
}

resource "aws_ecs_service" "frontend" {
  name            = "frontend"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.frontend.arn
  desired_count   = var.frontend_desired_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = aws_subnet.public[*].id
    security_groups  = [aws_security_group.frontend.id]
    assign_public_ip = true
  }
}
