# The Experiment API as a container-image Lambda behind an API Gateway HTTP API (payload format 2.0).
# Same code in local (Floci) and prod. scripts/infra-apply.sh creates the repository first, publishes the
# image (prod: pushed to ECR; local: built under its ECR URI), then creates the function and the gateway.
# The function runs in the private subnets with the app security group (Aurora). No NAT: it reaches
# the database, S3 (gateway endpoint in network.tf) and, in prod, Secrets Manager (interface endpoint in
# db_migrate.tf) only. SSM and other AWS APIs are not reachable.
# It also serves ProviderService (the Connectors page). Its secrets are Secrets Manager secrets named
# /<project>/<environment>/providers/<provider-id> (src/experimentapi/providers/secrets.go).

data "aws_caller_identity" "current" {}

data "aws_partition" "current" {}

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
      "s3:ListBucket",
    ]
    resources = [
      aws_s3_bucket.lake.arn,
      "${aws_s3_bucket.lake.arn}/*",
    ]
  }

  # Workspace cleanup deletes experiment shards only (src/internal/workspace). Candles are never deleted.
  statement {
    sid       = "ExperimentWorkspaceDelete"
    actions   = ["s3:DeleteObject"]
    resources = ["${aws_s3_bucket.lake.arn}/experiments/*"]
  }
}

data "aws_iam_policy_document" "experiment_api_secrets" {
  statement {
    sid = "ProviderSecrets"
    actions = [
      "secretsmanager:GetSecretValue",
      "secretsmanager:PutSecretValue",
      "secretsmanager:CreateSecret",
      "secretsmanager:DescribeSecret",
    ]
    resources = [
      "arn:${data.aws_partition.current.partition}:secretsmanager:${var.aws_region}:${data.aws_caller_identity.current.account_id}:secret:/${var.project}/${var.environment}/providers/*",
    ]
  }
}

resource "aws_iam_role_policy" "experiment_api_secrets" {
  name   = "${local.prefix}-experiment-api-secrets"
  role   = aws_iam_role.experiment_api.id
  policy = data.aws_iam_policy_document.experiment_api_secrets.json
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
      DATABASE_URL = local.app_database_url
      LAKE_BUCKET  = aws_s3_bucket.lake.bucket
      LAKE_BACKEND = "s3"
      # Run, Cancel and progress go through this state machine (experiment_workflow.tf).
      EXPERIMENT_SFN_ARN = aws_sfn_state_machine.experiment_run.arn
      # BackfillService (backfill_sfn.tf). BINANCE_BASE_URL is the fallback base URL of the provider, the same value the
      # ECS jobs read from SSM.
      BACKFILL_SFN_ARN = aws_sfn_state_machine.backfill.arn
      BINANCE_BASE_URL = aws_ssm_parameter.binance_base_url.value
      # Provider secrets: Secrets Manager under SSM_PREFIX (/<project>/<environment>), as the jobs use.
      SECRETS_BACKEND = "secretsmanager"
      SSM_PREFIX      = "/${var.project}/${var.environment}"
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
  api_id             = aws_apigatewayv2_api.experiment.id
  route_key          = "POST /tradingcopilot.v1.ExperimentService/{method}"
  authorization_type = "CUSTOM"
  authorizer_id      = aws_apigatewayv2_authorizer.origin_verify.id
  target             = "integrations/${aws_apigatewayv2_integration.experiment.id}"
}

# Connect-RPC unary calls: POST /tradingcopilot.v1.ProviderService/<Method>
resource "aws_apigatewayv2_route" "provider_rpc" {
  api_id             = aws_apigatewayv2_api.experiment.id
  route_key          = "POST /tradingcopilot.v1.ProviderService/{method}"
  authorization_type = "CUSTOM"
  authorizer_id      = aws_apigatewayv2_authorizer.origin_verify.id
  target             = "integrations/${aws_apigatewayv2_integration.experiment.id}"
}

# Connect-RPC unary calls: POST /tradingcopilot.v1.BackfillService/<Method> (backfill_sfn.tf)
resource "aws_apigatewayv2_route" "backfill_rpc" {
  api_id             = aws_apigatewayv2_api.experiment.id
  route_key          = "POST /tradingcopilot.v1.BackfillService/{method}"
  authorization_type = "CUSTOM"
  authorizer_id      = aws_apigatewayv2_authorizer.origin_verify.id
  target             = "integrations/${aws_apigatewayv2_integration.experiment.id}"
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

# CloudFront is the only caller. It sends this secret in x-origin-verify (frontend.tf origin "api"); the authorizer
# rejects any request without it. The secret lives in Terraform state (remote, encrypted) and in the authorizer's env.
resource "random_password" "origin_verify" {
  length  = 40
  special = false
}

data "archive_file" "origin_authorizer" {
  type        = "zip"
  source_file = "${path.module}/authorizer/index.py"
  output_path = "${path.module}/authorizer/.build/origin-authorizer.zip"
}

data "aws_iam_policy_document" "origin_authorizer_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "origin_authorizer" {
  name               = "${local.prefix}-origin-authorizer"
  assume_role_policy = data.aws_iam_policy_document.origin_authorizer_assume.json
}

resource "aws_iam_role_policy" "origin_authorizer_logs" {
  name = "${local.prefix}-origin-authorizer-logs"
  role = aws_iam_role.origin_authorizer.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["logs:CreateLogGroup", "logs:CreateLogStream", "logs:PutLogEvents"]
      Resource = "*"
    }]
  })
}

resource "aws_lambda_function" "origin_authorizer" {
  function_name    = "${local.prefix}-origin-authorizer"
  role             = aws_iam_role.origin_authorizer.arn
  runtime          = "python3.12"
  handler          = "index.handler"
  filename         = data.archive_file.origin_authorizer.output_path
  source_code_hash = data.archive_file.origin_authorizer.output_base64sha256
  timeout          = 3

  environment {
    variables = {
      ORIGIN_VERIFY = random_password.origin_verify.result
    }
  }
}

resource "aws_apigatewayv2_authorizer" "origin_verify" {
  api_id                            = aws_apigatewayv2_api.experiment.id
  authorizer_type                   = "REQUEST"
  authorizer_uri                    = aws_lambda_function.origin_authorizer.invoke_arn
  authorizer_payload_format_version = "2.0"
  enable_simple_responses           = true
  authorizer_result_ttl_in_seconds  = 0
  identity_sources                  = ["$request.header.x-origin-verify"]
  name                              = "origin-verify"
}

resource "aws_lambda_permission" "origin_authorizer_gateway" {
  statement_id  = "AllowApiGatewayAuthorizer"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.origin_authorizer.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.experiment.execution_arn}/authorizers/${aws_apigatewayv2_authorizer.origin_verify.id}"
}
