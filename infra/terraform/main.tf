# ---------------------------------------------------------------------------
# Data plane: S3 lake + SSM (+ shared task IAM). The relational control plane is Aurora (database.tf).
# No NAT, ALB, Secrets Manager, or always-on compute.
# ---------------------------------------------------------------------------

resource "aws_s3_bucket" "lake" {
  bucket = "${local.prefix}-lake"
}

resource "aws_s3_bucket_public_access_block" "lake" {
  bucket = aws_s3_bucket.lake.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "lake" {
  bucket = aws_s3_bucket.lake.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

# Optional lifecycle: expire incomplete multipart uploads (keeps idle tidy).
resource "aws_s3_bucket_lifecycle_configuration" "lake" {
  bucket = aws_s3_bucket.lake.id

  rule {
    id     = "abort-incomplete-multipart"
    status = "Enabled"

    abort_incomplete_multipart_upload {
      days_after_initiation = 7
    }

    filter {}
  }
}

# Prefer SSM over Secrets Manager (no per-secret monthly idle fee).
resource "aws_ssm_parameter" "binance_base_url" {
  name  = "/${var.project}/${var.environment}/BINANCE_BASE_URL"
  type  = "String"
  value = "https://data-api.binance.vision"
}

# Task role for Lambda and the ECS data-prep tasks (jobs_ecs.tf): S3 lake and SSM read.
data "aws_iam_policy_document" "task_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com", "ecs-tasks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "task" {
  name               = "${local.prefix}-task"
  assume_role_policy = data.aws_iam_policy_document.task_assume.json
}

data "aws_iam_policy_document" "task_data" {
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

  statement {
    sid = "SSMRead"
    actions = [
      "ssm:GetParameter",
      "ssm:GetParameters",
      "ssm:GetParametersByPath",
    ]
    resources = ["arn:aws:ssm:${var.aws_region}:*:parameter/${var.project}/${var.environment}/*"]
  }
}

resource "aws_iam_role_policy" "task_data" {
  name   = "${local.prefix}-task-data"
  role   = aws_iam_role.task.id
  policy = data.aws_iam_policy_document.task_data.json
}
