# Experiment run as a Step Functions workflow. The experiment-worker is a container-image Lambda (same
# phases as the experiment container, see docker/experiment-worker). The Experiment API starts, stops and
# describes executions (src/internal/experimentworkflow). Definition: experiment_fanout.tf.

resource "aws_ecr_repository" "experiment_worker" {
  name                 = "${local.prefix}-experiment-worker"
  image_tag_mutability = "MUTABLE"
  force_delete         = var.environment == "local"

  depends_on = [terraform_data.deploy_gate]
}

resource "aws_cloudwatch_log_group" "experiment_worker" {
  name              = "/aws/lambda/${local.prefix}-experiment-worker"
  retention_in_days = 14
}

# Dedicated role, like the API: the shared task role has no ENI permissions for a VPC-attached function.
resource "aws_iam_role" "experiment_worker" {
  name               = "${local.prefix}-experiment-worker"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume.json
}

resource "aws_iam_role_policy_attachment" "experiment_worker_vpc" {
  role       = aws_iam_role.experiment_worker.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaVPCAccessExecutionRole"
}

resource "aws_iam_role_policy" "experiment_worker" {
  name = "${local.prefix}-experiment-worker"
  role = aws_iam_role.experiment_worker.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Sid    = "S3Lake"
      Effect = "Allow"
      Action = ["s3:GetObject", "s3:PutObject", "s3:DeleteObject", "s3:ListBucket"]
      Resource = [
        aws_s3_bucket.lake.arn,
        "${aws_s3_bucket.lake.arn}/*",
      ]
    }]
  })
}

resource "aws_lambda_function" "experiment_worker" {
  function_name = "${local.prefix}-experiment-worker"
  role          = aws_iam_role.experiment_worker.arn
  package_type  = "Image"
  image_uri     = "${aws_ecr_repository.experiment_worker.repository_url}:${var.experiment_worker_image_tag}"
  timeout       = 900
  memory_size   = var.experiment_worker_memory_mb

  ephemeral_storage {
    size = var.experiment_worker_ephemeral_mb
  }

  vpc_config {
    subnet_ids         = aws_subnet.private[*].id
    security_group_ids = [aws_security_group.app.id]
  }

  environment {
    variables = {
      DATABASE_URL          = local.database_url
      LAKE_BUCKET           = aws_s3_bucket.lake.bucket
      LAKE_BACKEND          = "s3"
      DATA_DIR              = "/tmp"
      EXPERIMENT_BATCH_SIZE = tostring(var.experiment_batch_size)
    }
  }

  depends_on = [
    aws_iam_role_policy_attachment.experiment_worker_vpc,
    aws_iam_role_policy.experiment_worker,
    aws_cloudwatch_log_group.experiment_worker,
  ]
}

data "aws_iam_policy_document" "sfn_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["states.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "experiment_sfn" {
  name               = "${local.prefix}-sfn-experiment"
  assume_role_policy = data.aws_iam_policy_document.sfn_assume.json
}

resource "aws_iam_role_policy" "experiment_sfn" {
  name = "${local.prefix}-sfn-experiment"
  role = aws_iam_role.experiment_sfn.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Sid      = "InvokeExperimentWorker"
      Effect   = "Allow"
      Action   = ["lambda:InvokeFunction"]
      Resource = [aws_lambda_function.experiment_worker.arn]
    }]
  })
}

resource "aws_sfn_state_machine" "experiment_run" {
  name       = "${local.prefix}-experiment-run"
  role_arn   = aws_iam_role.experiment_sfn.arn
  definition = local.experiment_sfn_definition
}

# The Experiment API starts, lists, stops and describes executions of this state machine only.
resource "aws_iam_role_policy" "experiment_api_sfn" {
  name = "${local.prefix}-experiment-api-sfn"
  role = aws_iam_role.experiment_api.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Sid    = "ExperimentRunWorkflow"
      Effect = "Allow"
      Action = [
        "states:StartExecution",
        "states:StopExecution",
        "states:ListExecutions",
        "states:DescribeExecution",
      ]
      Resource = [
        aws_sfn_state_machine.experiment_run.arn,
        "${replace(aws_sfn_state_machine.experiment_run.arn, ":stateMachine:", ":execution:")}:*",
      ]
    }]
  })
}
