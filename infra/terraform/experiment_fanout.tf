# Step 6 — experiment instance fan-out (SFN Inline Map + experiment-worker Lambda).
# Optional ECS cluster for Fargate Spot long-tail (EXPERIMENT_RUN_HEAVY=1); not wired to RunTask yet.

variable "experiment_batch_size" {
  type    = number
  default = 64
}

variable "experiment_map_max_concurrency" {
  type    = number
  default = 200
}

variable "enable_experiment_lambda" {
  description = "Create experiment-worker Lambda (requires experiment_worker_zip_path)"
  type        = bool
  default     = false
}

variable "experiment_worker_zip_path" {
  description = "Path to experiment-worker bootstrap zip (go build -o bootstrap …)"
  type        = string
  default     = ""
}

# Optional future: ECS cluster for Fargate Spot experiment heavy runs.
# resource "aws_ecs_cluster" "experiment" {
#   name = "${local.prefix}-experiment"
#   setting {
#     name  = "containerInsights"
#     value = "disabled"
#   }
# }

data "aws_iam_policy_document" "experiment_sfn_assume" {
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
  assume_role_policy = data.aws_iam_policy_document.experiment_sfn_assume.json
}

data "aws_iam_policy_document" "experiment_sfn" {
  statement {
    sid = "InvokeExperimentWorker"
    actions = [
      "lambda:InvokeFunction",
    ]
    resources = var.enable_experiment_lambda ? [aws_lambda_function.experiment_worker[0].arn] : ["*"]
  }

  statement {
    sid = "Logs"
    actions = [
      "logs:CreateLogDelivery",
      "logs:GetLogDelivery",
      "logs:UpdateLogDelivery",
      "logs:DeleteLogDelivery",
      "logs:ListLogDeliveries",
      "logs:PutResourcePolicy",
      "logs:DescribeResourcePolicies",
      "logs:DescribeLogGroups",
    ]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "experiment_sfn" {
  name   = "${local.prefix}-sfn-experiment"
  role   = aws_iam_role.experiment_sfn.id
  policy = data.aws_iam_policy_document.experiment_sfn.json
}

resource "aws_iam_role_policy_attachment" "experiment_worker_logs" {
  count      = var.enable_experiment_lambda ? 1 : 0
  role       = aws_iam_role.task.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_lambda_function" "experiment_worker" {
  count = var.enable_experiment_lambda ? 1 : 0

  function_name = "${local.prefix}-experiment-worker"
  role          = aws_iam_role.task.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  filename      = var.experiment_worker_zip_path
  timeout       = 900
  memory_size   = 1024

  environment {
    variables = {
      VENUE_SIMULATE     = "1"
      # Experiment catalog is Postgres. DynamoDB is not required for this worker.
      EXPERIMENT_CATALOG = "postgres"
      DATABASE_URL       = var.experiment_database_url
    }
  }
}

locals {
  experiment_worker_arn = var.enable_experiment_lambda ? aws_lambda_function.experiment_worker[0].arn : "arn:aws:lambda:${var.aws_region}:000000000000:function:experiment-worker-placeholder"
  experiment_sfn_definition = jsonencode({
    Comment = "Experiment simulate batches via experiment-worker Lambda"
    StartAt = "MapBatches"
    States = {
      MapBatches = {
        Type           = "Map"
        ItemsPath      = "$.batches"
        MaxConcurrency = var.experiment_map_max_concurrency
        Iterator = {
          StartAt = "SimulateBatch"
          States = {
            SimulateBatch = {
              Type     = "Task"
              Resource = "arn:aws:states:::lambda:invoke"
              Parameters = {
                FunctionName = local.experiment_worker_arn
                Payload = {
                  "experimentId.$"     = "$$.Execution.Input.experimentId"
                  "instanceIds.$" = "$"
                  "dataDir.$"     = "$$.Execution.Input.dataDir"
                }
              }
              End = true
            }
          }
        }
        End = true
      }
    }
  })
}

resource "aws_sfn_state_machine" "experiment_run" {
  name     = "${local.prefix}-experiment-run"
  role_arn = aws_iam_role.experiment_sfn.arn

  definition = local.experiment_sfn_definition
}
