# The Experiment API as a container-image Lambda behind an API Gateway HTTP API (payload format 2.0).
# Same code in local (Floci) and prod. scripts/infra-apply.sh creates the repository first, publishes the
# image (prod: pushed to ECR; local: built under its ECR URI), then creates the function and the gateway.
# The function runs in the private subnets with the app security group (Aurora). No NAT: it reaches
# the database and S3 (gateway endpoint in network.tf) only. SSM and other AWS APIs are not reachable.

resource "aws_ecr_repository" "experiment_api" {
  name                 = "${local.prefix}-experiment-api"
  image_tag_mutability = "MUTABLE"
  force_delete         = var.environment == "local"

  depends_on = [terraform_data.deploy_gate]
}

resource "aws_cloudwatch_log_group" "experiment_api" {
  name              = "/aws/lambda/${local.prefix}-experiment-api"
  retention_in_days = 14
}

data "aws_iam_policy_document" "lambda_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

# Dedicated role: the shared task role has no ENI permissions, which a VPC-attached function needs.
resource "aws_iam_role" "experiment_api" {
  name               = "${local.prefix}-experiment-api"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume.json
}

# Logs and the VPC ENI permissions (ec2:CreateNetworkInterface and friends).
resource "aws_iam_role_policy_attachment" "experiment_api_vpc" {
  role       = aws_iam_role.experiment_api.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaVPCAccessExecutionRole"
}

data "aws_iam_policy_document" "experiment_api" {
  statement {
    sid = "S3Lake"
    actions = [
      "s3:GetObject",
      "s3:PutObject",
      "s3:DeleteObject",
      "s3:ListBucket",
    ]
    resources = [
      aws_s3_bucket.lake.arn,
      "${aws_s3_bucket.lake.arn}/*",
    ]
  }
}

resource "aws_iam_role_policy" "experiment_api" {
  name   = "${local.prefix}-experiment-api"
  role   = aws_iam_role.experiment_api.id
  policy = data.aws_iam_policy_document.experiment_api.json
}

resource "aws_lambda_function" "experiment_api" {
  function_name = "${local.prefix}-experiment-api"
  role          = aws_iam_role.experiment_api.arn
  package_type  = "Image"
  image_uri     = "${aws_ecr_repository.experiment_api.repository_url}:${var.api_image_tag}"
  timeout       = 29 # below the API Gateway integration limit of 30 s
  memory_size   = 512

  vpc_config {
    subnet_ids         = aws_subnet.private[*].id
    security_group_ids = [aws_security_group.app.id]
  }

  environment {
    variables = {
      DATABASE_URL = local.database_url
      LAKE_BUCKET  = aws_s3_bucket.lake.bucket
      LAKE_BACKEND = "s3"
    }
  }

  depends_on = [
    aws_iam_role_policy_attachment.experiment_api_vpc,
    aws_iam_role_policy.experiment_api,
    aws_cloudwatch_log_group.experiment_api,
  ]
}

resource "aws_apigatewayv2_api" "experiment" {
  name          = "${local.prefix}-experiment-api"
  protocol_type = "HTTP"

  cors_configuration {
    allow_origins = var.api_cors_allow_origins
    allow_methods = ["POST", "OPTIONS"]
    allow_headers = [
      "content-type",
      "connect-protocol-version",
      "connect-timeout-ms",
      "accept-encoding",
      "authorization",
    ]
  }
}

resource "aws_apigatewayv2_integration" "experiment" {
  api_id                 = aws_apigatewayv2_api.experiment.id
  integration_type       = "AWS_PROXY"
  integration_method     = "POST"
  integration_uri        = aws_lambda_function.experiment_api.invoke_arn
  payload_format_version = "2.0"
}

# Connect-RPC unary calls: POST /tradingcopilot.v1.ExperimentService/<Method>
resource "aws_apigatewayv2_route" "experiment_rpc" {
  api_id    = aws_apigatewayv2_api.experiment.id
  route_key = "POST /tradingcopilot.v1.ExperimentService/{method}"
  target    = "integrations/${aws_apigatewayv2_integration.experiment.id}"
}

resource "aws_apigatewayv2_stage" "default" {
  api_id      = aws_apigatewayv2_api.experiment.id
  name        = "$default"
  auto_deploy = true
}

resource "aws_lambda_permission" "experiment_api_gateway" {
  statement_id  = "AllowApiGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.experiment_api.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.experiment.execution_arn}/*/*"
}
