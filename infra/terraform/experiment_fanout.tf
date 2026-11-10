# Experiment run workflow: tunables and the Step Functions definition (Prepare -> Map(Simulate) -> Finalize).
# The resources are in experiment_workflow.tf.

variable "experiment_batch_size" {
  description = "Instances per simulate batch (EXPERIMENT_BATCH_SIZE in the worker)."
  type        = number
  default     = 64
}

variable "experiment_map_max_concurrency" {
  description = "Simulate batches running in parallel in the Map state."
  type        = number
  default     = 200
}

variable "experiment_worker_image_tag" {
  description = "Tag of the experiment-worker image in ECR. scripts/infra-apply.sh builds and pushes it before the function is created."
  type        = string
  default     = "latest"
}

variable "experiment_worker_memory_mb" {
  type    = number
  default = 3008
}

variable "experiment_worker_ephemeral_mb" {
  description = "/tmp size of the worker (512 to 10240 MB)."
  type        = number
  default     = 2048
}

locals {
  # Every Lambda task invokes the worker with a Payload; the Retry covers Lambda service faults only.
  experiment_worker_retry = [{
    ErrorEquals = [
      "Lambda.ServiceException",
      "Lambda.AWSLambdaException",
      "Lambda.SdkClientException",
      "Lambda.TooManyRequestsException",
    ]
    IntervalSeconds = 2
    MaxAttempts     = 3
    BackoffRate     = 2
  }]

  experiment_worker_catch = [{
    ErrorEquals = ["States.ALL"]
    Next        = "MarkFailed"
    ResultPath  = "$.error"
  }]

  experiment_sfn_definition = jsonencode({
    Comment        = "Experiment run: prepare, simulate batches in a Map, finalize. Any failure marks the experiment failed."
    StartAt        = "Prepare"
    TimeoutSeconds = 86400
    States = {
      Prepare = {
        Type     = "Task"
        Resource = "arn:aws:states:::lambda:invoke"
        Parameters = {
          FunctionName = aws_lambda_function.experiment_worker.arn
          Payload = {
            phase             = "prepare"
            "experiment_id.$" = "$.experiment_id"
            "retry_failed.$"  = "$.retry_failed"
          }
        }
        ResultSelector = { "batches.$" = "$.Payload.batches" }
        ResultPath     = "$.prepared"
        Retry          = local.experiment_worker_retry
        Catch          = local.experiment_worker_catch
        Next           = "Simulate"
      }
      Simulate = {
        Type           = "Map"
        ItemsPath      = "$.prepared.batches"
        MaxConcurrency = var.experiment_map_max_concurrency
        ItemSelector = {
          phase             = "simulate"
          "experiment_id.$" = "$.experiment_id"
          "instance_ids.$"  = "$$.Map.Item.Value"
        }
        ResultPath = null
        Iterator = {
          StartAt = "SimulateBatch"
          States = {
            SimulateBatch = {
              Type     = "Task"
              Resource = "arn:aws:states:::lambda:invoke"
              Parameters = {
                FunctionName = aws_lambda_function.experiment_worker.arn
                "Payload.$"  = "$"
              }
              ResultPath = null
              Retry      = local.experiment_worker_retry
              End        = true
            }
          }
        }
        Catch = local.experiment_worker_catch
        Next  = "Finalize"
      }
      Finalize = {
        Type     = "Task"
        Resource = "arn:aws:states:::lambda:invoke"
        Parameters = {
          FunctionName = aws_lambda_function.experiment_worker.arn
          Payload = {
            phase             = "finalize"
            "experiment_id.$" = "$.experiment_id"
          }
        }
        ResultPath = null
        Retry      = local.experiment_worker_retry
        Catch      = local.experiment_worker_catch
        End        = true
      }
      MarkFailed = {
        Type     = "Task"
        Resource = "arn:aws:states:::lambda:invoke"
        Parameters = {
          FunctionName = aws_lambda_function.experiment_worker.arn
          Payload = {
            phase             = "fail"
            "experiment_id.$" = "$.experiment_id"
            "error.$"         = "$.error.Cause"
          }
        }
        ResultPath = null
        Next       = "Failed"
      }
      Failed = {
        Type  = "Fail"
        Error = "ExperimentFailed"
        Cause = "An experiment phase failed; the experiment is marked failed."
      }
    }
  })
}
