# The web app: the Vite SPA as static files in a private S3 bucket, served by CloudFront on one origin.
#   /tradingcopilot.v1.*  -> API Gateway (api_lambda.tf), no caching, all methods
#   everything else       -> S3 (OAC in prod), cached; a viewer-request function maps extensionless paths to /index.html
# Same origin for the browser, so the Experiment API needs no CORS for this site.
# scripts/frontend-deploy.sh builds the SPA and uploads dist/ after the apply.
#
# local (Floci): CloudFront and S3 are reached through Floci's own host and internal port (4566), over http.
# Floci does not enforce OAC, and the S3 origin is a custom origin with the bucket as origin path.
# No custom domain or ACM certificate yet (follow-up): the cloudfront_default_certificate is used.

locals {
  frontend_api_host = local.is_local ? "127.0.0.1" : replace(aws_apigatewayv2_api.experiment.api_endpoint, "https://", "")
  frontend_s3_host  = local.is_local ? "127.0.0.1" : aws_s3_bucket.frontend.bucket_regional_domain_name
}

resource "aws_s3_bucket" "frontend" {
  bucket        = "${local.prefix}-frontend"
  force_destroy = local.is_local

  depends_on = [terraform_data.deploy_gate]
}

resource "aws_s3_bucket_public_access_block" "frontend" {
  bucket = aws_s3_bucket.frontend.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_cloudfront_origin_access_control" "frontend" {
  name                              = "${local.prefix}-frontend"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

# Paths without a file extension (/, /experiments/42) are SPA routes: serve index.html. Paths with an
# extension (/assets/x.js, /favicon.ico) pass through; a missing file answers 403/404 from S3.
# Floci 2.4.0 stores and associates the function but does not execute it: locally, SPA deep links are 404.
resource "aws_cloudfront_function" "spa_fallback" {
  name    = "${local.prefix}-spa-fallback"
  runtime = "cloudfront-js-2.0"
  publish = true
  code    = <<-EOT
    function handler(event) {
      var request = event.request;
      var last = request.uri.substring(request.uri.lastIndexOf('/') + 1);
      if (last.indexOf('.') === -1) {
        request.uri = '/index.html';
      }
      return request;
    }
  EOT
}

resource "aws_cloudfront_distribution" "frontend" {
  enabled             = true
  is_ipv6_enabled     = true
  default_root_object = "index.html"
  price_class         = "PriceClass_100"
  comment             = "${local.prefix} web app"

  origin {
    origin_id                = "s3"
    domain_name              = local.frontend_s3_host
    origin_path              = local.is_local ? "/${aws_s3_bucket.frontend.bucket}" : null
    origin_access_control_id = local.is_local ? null : aws_cloudfront_origin_access_control.frontend.id

    dynamic "custom_origin_config" {
      for_each = local.is_local ? [1] : []
      content {
        http_port              = 4566
        https_port             = 443
        origin_protocol_policy = "http-only"
        origin_ssl_protocols   = ["TLSv1.2"]
      }
    }
  }

  origin {
    origin_id   = "api"
    domain_name = local.frontend_api_host
    # The stage is $default (api_lambda.tf). Floci serves API Gateway under its execute-api path.
    origin_path = local.is_local ? "/_aws/execute-api/${aws_apigatewayv2_api.experiment.id}/$default" : null

    custom_origin_config {
      http_port              = 4566
      https_port             = 443
      origin_protocol_policy = local.is_local ? "http-only" : "https-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }

  default_cache_behavior {
    target_origin_id       = "s3"
    viewer_protocol_policy = local.is_local ? "allow-all" : "redirect-to-https"
    allowed_methods        = ["GET", "HEAD"]
    cached_methods         = ["GET", "HEAD"]
    compress               = true
    # Managed-CachingOptimized
    cache_policy_id = "658327ea-f89d-4fab-a63d-7f88639e58f6"

    # Client-side routes (BrowserRouter) have no object in S3. Only this behavior rewrites: the API keeps
    # its own 403/404 (Connect NotFound and PermissionDenied), so no error mapping is set on the distribution.
    function_association {
      event_type   = "viewer-request"
      function_arn = aws_cloudfront_function.spa_fallback.arn
    }
  }

  ordered_cache_behavior {
    path_pattern           = "/tradingcopilot.v1.*"
    target_origin_id       = "api"
    viewer_protocol_policy = local.is_local ? "allow-all" : "redirect-to-https"
    allowed_methods        = ["GET", "HEAD", "OPTIONS", "PUT", "POST", "PATCH", "DELETE"]
    cached_methods         = ["GET", "HEAD"]
    # Managed-CachingDisabled and Managed-AllViewerExceptHostHeader: Connect calls are dynamic and
    # the origin must see the viewer's headers, but not the CloudFront host.
    cache_policy_id          = "4135ea2d-6df8-4a3e-a2b4-cb4efb2cab1d"
    origin_request_policy_id = "b689b0a8-53d0-40ab-baf2-68738e2966ac"
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    cloudfront_default_certificate = true
  }
}

# Only this distribution may read objects (OAC). Prod only in effect: Floci does not enforce it.
data "aws_iam_policy_document" "frontend_bucket" {
  statement {
    sid       = "CloudFrontRead"
    actions   = ["s3:GetObject"]
    resources = ["${aws_s3_bucket.frontend.arn}/*"]

    principals {
      type        = "Service"
      identifiers = ["cloudfront.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "AWS:SourceArn"
      values   = [aws_cloudfront_distribution.frontend.arn]
    }
  }
}

resource "aws_s3_bucket_policy" "frontend" {
  bucket = aws_s3_bucket.frontend.id
  policy = data.aws_iam_policy_document.frontend_bucket.json

  depends_on = [aws_s3_bucket_public_access_block.frontend]
}
