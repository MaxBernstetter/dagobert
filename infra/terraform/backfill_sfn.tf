# Backfill of one market over a time range: ingest, then the technical-indicator job.
# Input (one execution per provider/base/quote): {"provider":"binance","base":"BTC","quote":"USDT","from":<ms>,"to":<ms>}
# The orchestrator passed the same values to its containers (backfill.go): to = candle close, from = open of the first bar.
# Exit codes: 0 and 2 (already stored / nothing to do) continue; 1 or any ECS failure ends the execution as FAILED.
# Exit 2 arrives as a States.TaskFailed error whose Cause carries the ExitCode (see local.backfill_sfn_definition).

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

  # Why the Catch: runTask.sync does not return a non-zero container exit to a Choice. It fails the Task state
  # with States.TaskFailed, and the Cause is a JSON string of the task description (ExitCode per container).
  # So exit 0 is the normal output and exit 2 ("already stored") is recovered by matching the Cause:
  # StringMatches on "ExitCode":2 (the pattern assumes compact JSON with PascalCase keys). Any other cause
  # (exit 1, other ECS failures, States.ALL) ends the execution FAILED. Unverified on Floci and on AWS.
  backfill_sfn_definition = jsonencode({
    Comment = "Backfill one market: ingest klines into the lake, then build the technical indicators"
    StartAt = "Ingest"
    States = {
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
            }]
          }
        }
        ResultPath     = "$.ingest"
        TimeoutSeconds = 14400
        Catch = [
          { ErrorEquals = ["States.TaskFailed"], ResultPath = "$.error", Next = "CheckIngestError" },
          { ErrorEquals = ["States.ALL"], ResultPath = "$.error", Next = "IngestFailed" },
        ]
        Next = "Indicator"
      }
      CheckIngestError = {
        Type = "Choice"
        Choices = [{
          Or = [
            { Variable = "$.error.Cause", StringMatches = "*\"ExitCode\":2,*" },
            { Variable = "$.error.Cause", StringMatches = "*\"ExitCode\":2}*" },
          ]
          Next = "Indicator"
        }]
        Default = "IngestFailed"
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
              ]
            }]
          }
        }
        ResultPath     = "$.indicator"
        TimeoutSeconds = 14400
        Catch = [
          { ErrorEquals = ["States.TaskFailed"], ResultPath = "$.error", Next = "CheckIndicatorError" },
          { ErrorEquals = ["States.ALL"], ResultPath = "$.error", Next = "IndicatorFailed" },
        ]
        Next = "Done"
      }
      CheckIndicatorError = {
        Type = "Choice"
        Choices = [{
          Or = [
            { Variable = "$.error.Cause", StringMatches = "*\"ExitCode\":2,*" },
            { Variable = "$.error.Cause", StringMatches = "*\"ExitCode\":2}*" },
          ]
          Next = "Done"
        }]
        Default = "IndicatorFailed"
      }
      Done = {
        Type = "Succeed"
      }
      IngestFailed = {
        Type  = "Fail"
        Error = "IngestFailed"
        Cause = "The ingest task failed or exited with a code other than 0 or 2"
      }
      IndicatorFailed = {
        Type  = "Fail"
        Error = "IndicatorFailed"
        Cause = "The indicator task failed or exited with a code other than 0 or 2"
      }
    }
  })
}

resource "aws_sfn_state_machine" "backfill" {
  name       = "${local.prefix}-backfill"
  role_arn   = aws_iam_role.backfill_sfn.arn
  definition = local.backfill_sfn_definition
}
