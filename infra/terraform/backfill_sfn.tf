# Backfill of one market over a time range: ingest, then the technical-indicator job.
# Input (one execution per provider/base/quote): {"provider":"binance","base":"BTC","quote":"USDT","from":<ms>,"to":<ms>}
# with an optional "binance_base_url": the Backfill API sends the saved provider base URL. Without it, the state
# machine uses the SSM value (HasBaseUrl), so scripts/backfill-start.sh and executions started before it still work.
# to = candle close, from = open of the first bar.
# Each job runs as ecs:runTask.sync. The containers have COMPOSE_OK_ON_ABORT=1 (jobs_ecs.tf), so exit 2 (already
# stored) exits 0 and the task succeeds. Only a task that exits 1 (or any other non-zero code) fails the Task state
# with States.TaskFailed and ends the execution FAILED.

data "aws_iam_policy_document" "backfill_sfn_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["states.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "backfill_sfn" {
  name               = "${local.prefix}-sfn-backfill"
  assume_role_policy = data.aws_iam_policy_document.backfill_sfn_assume.json
}

data "aws_iam_policy_document" "backfill_sfn" {
  # RunTask.sync: start the task, then wait for it to stop.
  statement {
    sid = "RunJobTasks"
    actions = [
      "ecs:RunTask",
      "ecs:StopTask",
      "ecs:DescribeTasks",
    ]
    resources = [
      "${aws_ecs_task_definition.ingest.arn_without_revision}:*",
      "${aws_ecs_task_definition.indicator.arn_without_revision}:*",
      aws_ecs_cluster.jobs.arn,
    ]
  }

  # The .sync integration waits on the managed EventBridge rule that ECS task state changes feed.
  statement {
    sid = "SyncEventRule"
    actions = [
      "events:PutTargets",
      "events:PutRule",
      "events:DescribeRule",
    ]
    resources = ["arn:aws:events:${var.aws_region}:*:rule/StepFunctionsGetEventsForECSTaskRule"]
  }

  statement {
    sid     = "PassJobRoles"
    actions = ["iam:PassRole"]
    resources = [
      aws_iam_role.jobs_execution.arn,
      aws_iam_role.task.arn,
    ]
    condition {
      test     = "StringEquals"
      variable = "iam:PassedToService"
      values   = ["ecs-tasks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role_policy" "backfill_sfn" {
  name   = "${local.prefix}-sfn-backfill"
  role   = aws_iam_role.backfill_sfn.id
  policy = data.aws_iam_policy_document.backfill_sfn.json
}

locals {
  jobs_network = {
    AwsvpcConfiguration = {
      Subnets        = aws_subnet.public[*].id
      SecurityGroups = [aws_security_group.jobs.id]
      AssignPublicIp = "ENABLED"
    }
  }

  # runTask.sync returns when the task stops. Exit 0 continues to the next state. Any failure (exit 1, a task that
  # could not start, or States.ALL) ends the execution FAILED through the Catch. Exit 2 never reaches this layer:
  # the container maps it to 0 (COMPOSE_OK_ON_ABORT=1).
  backfill_sfn_definition = jsonencode({
    Comment = "Backfill one market: ingest klines into the lake, then build the technical indicators"
    StartAt = "HasBaseUrl"
    States = {
      HasBaseUrl = {
        Type    = "Choice"
        Choices = [{ Variable = "$.binance_base_url", IsPresent = true, Next = "Ingest" }]
        Default = "DefaultBaseUrl"
      }
      # The SSM value the ECS jobs read by default (BINANCE_BASE_URL, main.tf).
      DefaultBaseUrl = {
        Type       = "Pass"
        Result     = aws_ssm_parameter.binance_base_url.value
        ResultPath = "$.binance_base_url"
        Next       = "Ingest"
      }
      Ingest = {
        Type     = "Task"
        Resource = "arn:aws:states:::ecs:runTask.sync"
        Parameters = {
          LaunchType           = "FARGATE"
          Cluster              = aws_ecs_cluster.jobs.arn
          TaskDefinition       = aws_ecs_task_definition.ingest.arn
          NetworkConfiguration = local.jobs_network
          Overrides = {
            ContainerOverrides = [{
              Name        = "ingest"
              "Command.$" = "States.Array('-from', States.Format('{}', $.from), '-to', States.Format('{}', $.to), '-base', $.base, '-quote', $.quote, '-provider', $.provider)"
              Environment = [
                { Name = "BINANCE_BASE_URL", "Value.$" = "$.binance_base_url" },
              ]
            }]
          }
        }
        ResultPath     = "$.ingest"
        TimeoutSeconds = 14400
        Catch = [
          { ErrorEquals = ["States.ALL"], ResultPath = "$.error", Next = "IngestFailed" },
        ]
        Next = "Indicator"
      }
      Indicator = {
        Type     = "Task"
        Resource = "arn:aws:states:::ecs:runTask.sync"
        Parameters = {
          LaunchType           = "FARGATE"
          Cluster              = aws_ecs_cluster.jobs.arn
          TaskDefinition       = aws_ecs_task_definition.indicator.arn
          NetworkConfiguration = local.jobs_network
          Overrides = {
            ContainerOverrides = [{
              Name        = "indicator"
              "Command.$" = "States.Array(States.Format('{}{}', $.base, $.quote), States.Format('{}', $.from), States.Format('{}', $.to))"
              Environment = [
                { Name = "SOURCE", Value = "backfill" },
                { Name = "INGEST_PROVIDER", "Value.$" = "$.provider" },
                { Name = "BINANCE_BASE_URL", "Value.$" = "$.binance_base_url" },
              ]
            }]
          }
        }
        ResultPath     = "$.indicator"
        TimeoutSeconds = 14400
        Catch = [
          { ErrorEquals = ["States.ALL"], ResultPath = "$.error", Next = "IndicatorFailed" },
        ]
        Next = "Done"
      }
      Done = {
        Type = "Succeed"
      }
      IngestFailed = {
        Type  = "Fail"
        Error = "IngestFailed"
        Cause = "The ingest task failed: it exited with a code other than 0 or 2, or did not start. Its CloudWatch log group /ecs/${local.prefix}-ingest has the DETAIL line."
      }
      IndicatorFailed = {
        Type  = "Fail"
        Error = "IndicatorFailed"
        Cause = "The indicator task failed: it exited with a code other than 0 or 2, or did not start. Its CloudWatch log group /ecs/${local.prefix}-indicator has the DETAIL line."
      }
    }
  })
}

resource "aws_sfn_state_machine" "backfill" {
  name       = "${local.prefix}-backfill"
  role_arn   = aws_iam_role.backfill_sfn.arn
  definition = local.backfill_sfn_definition
}

# The Backfill API (experimentapi, BackfillService) starts, describes, lists and stops executions of this state
# machine only. The Experiment API Lambda runs it (api_lambda.tf).
resource "aws_iam_role_policy" "experiment_api_backfill" {
  name = "${local.prefix}-experiment-api-backfill"
  role = aws_iam_role.experiment_api.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Sid    = "BackfillWorkflow"
      Effect = "Allow"
      Action = [
        "states:StartExecution",
        "states:StopExecution",
        "states:ListExecutions",
        "states:DescribeExecution",
      ]
      Resource = [
        aws_sfn_state_machine.backfill.arn,
        "${replace(aws_sfn_state_machine.backfill.arn, ":stateMachine:", ":execution:")}:*",
      ]
    }]
  })
}
