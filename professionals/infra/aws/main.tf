# =============================================================================
# Terraform – Professionals Vertical (AWS)
# Independent deployment; no shared state with control-plane.
# =============================================================================

terraform {
  required_version = ">= 1.6.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  # Uncomment and configure for remote state:
  # backend "s3" {
  #   bucket         = "pymes-terraform-state"
  #   key            = "professionals/terraform.tfstate"
  #   region         = "us-east-1"
  #   dynamodb_table = "pymes-terraform-locks"
  #   encrypt        = true
  # }
}

provider "aws" {
  region = var.aws_region
}

# -----------------------------------------------------------------------------
# Locals
# -----------------------------------------------------------------------------

locals {
  prefix = "${var.project_name}-${var.environment}"

  frontend_bucket = var.frontend_bucket_name != "" ? var.frontend_bucket_name : "${local.prefix}-frontend"

  common_tags = {
    Project     = var.project_name
    Vertical    = "professionals"
    Environment = var.environment
    ManagedBy   = "terraform"
  }
}

# =============================================================================
# SSM Parameter Store – Secrets & Configuration
# =============================================================================

resource "aws_ssm_parameter" "database_url" {
  name  = "/${local.prefix}/DATABASE_URL"
  type  = "SecureString"
  value = "CHANGE_ME"

  tags = local.common_tags

  lifecycle {
    ignore_changes = [value]
  }
}

resource "aws_ssm_parameter" "control_plane_url" {
  name  = "/${local.prefix}/CONTROL_PLANE_URL"
  type  = "String"
  value = "CHANGE_ME"

  tags = local.common_tags

  lifecycle {
    ignore_changes = [value]
  }
}

resource "aws_ssm_parameter" "internal_service_token" {
  name  = "/${local.prefix}/INTERNAL_SERVICE_TOKEN"
  type  = "SecureString"
  value = "CHANGE_ME"

  tags = local.common_tags

  lifecycle {
    ignore_changes = [value]
  }
}

resource "aws_ssm_parameter" "clerk_issuer" {
  name  = "/${local.prefix}/CLERK_ISSUER"
  type  = "String"
  value = "CHANGE_ME"

  tags = local.common_tags

  lifecycle {
    ignore_changes = [value]
  }
}

resource "aws_ssm_parameter" "clerk_jwks_url" {
  name  = "/${local.prefix}/CLERK_JWKS_URL"
  type  = "String"
  value = "CHANGE_ME"

  tags = local.common_tags

  lifecycle {
    ignore_changes = [value]
  }
}

resource "aws_ssm_parameter" "gemini_api_key" {
  name  = "/${local.prefix}/GEMINI_API_KEY"
  type  = "SecureString"
  value = "CHANGE_ME"

  tags = local.common_tags

  lifecycle {
    ignore_changes = [value]
  }
}

resource "aws_ssm_parameter" "gemini_model" {
  name  = "/${local.prefix}/GEMINI_MODEL"
  type  = "String"
  value = "gemini-2.0-flash"

  tags = local.common_tags

  lifecycle {
    ignore_changes = [value]
  }
}

# =============================================================================
# IAM – Lambda Execution Roles
# =============================================================================

data "aws_iam_policy_document" "lambda_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

# -- Backend Role --------------------------------------------------------------

resource "aws_iam_role" "backend_lambda" {
  name               = "${local.prefix}-backend-lambda"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume.json
  tags               = local.common_tags
}

resource "aws_iam_role_policy_attachment" "backend_basic" {
  role       = aws_iam_role.backend_lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

data "aws_iam_policy_document" "backend_ssm" {
  statement {
    actions = [
      "ssm:GetParameter",
      "ssm:GetParameters",
      "ssm:GetParametersByPath",
    ]
    resources = [
      "arn:aws:ssm:${var.aws_region}:*:parameter/${local.prefix}/*",
    ]
  }
}

resource "aws_iam_role_policy" "backend_ssm" {
  name   = "${local.prefix}-backend-ssm"
  role   = aws_iam_role.backend_lambda.id
  policy = data.aws_iam_policy_document.backend_ssm.json
}

# -- AI Role -------------------------------------------------------------------

resource "aws_iam_role" "ai_lambda" {
  name               = "${local.prefix}-ai-lambda"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume.json
  tags               = local.common_tags
}

resource "aws_iam_role_policy_attachment" "ai_basic" {
  role       = aws_iam_role.ai_lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

data "aws_iam_policy_document" "ai_ssm" {
  statement {
    actions = [
      "ssm:GetParameter",
      "ssm:GetParameters",
      "ssm:GetParametersByPath",
    ]
    resources = [
      "arn:aws:ssm:${var.aws_region}:*:parameter/${local.prefix}/*",
    ]
  }
}

resource "aws_iam_role_policy" "ai_ssm" {
  name   = "${local.prefix}-ai-ssm"
  role   = aws_iam_role.ai_lambda.id
  policy = data.aws_iam_policy_document.ai_ssm.json
}

# =============================================================================
# CloudWatch Log Groups (created before Lambdas to control retention)
# =============================================================================

resource "aws_cloudwatch_log_group" "backend" {
  name              = "/aws/lambda/${local.prefix}-backend"
  retention_in_days = 30
  tags              = local.common_tags
}

resource "aws_cloudwatch_log_group" "ai" {
  name              = "/aws/lambda/${local.prefix}-ai"
  retention_in_days = 30
  tags              = local.common_tags
}

# =============================================================================
# Lambda Functions
# =============================================================================

resource "aws_lambda_function" "backend" {
  function_name = "${local.prefix}-backend"
  package_type  = "Image"
  image_uri     = var.backend_lambda_image_uri
  role          = aws_iam_role.backend_lambda.arn
  memory_size   = var.backend_lambda_memory
  timeout       = var.backend_lambda_timeout
  architectures = ["arm64"]

  environment {
    variables = {
      PORT                   = "8081"
      DATABASE_URL           = aws_ssm_parameter.database_url.value
      CONTROL_PLANE_URL      = aws_ssm_parameter.control_plane_url.value
      INTERNAL_SERVICE_TOKEN = aws_ssm_parameter.internal_service_token.value
      JWT_ISSUER             = aws_ssm_parameter.clerk_issuer.value
      JWKS_URL               = aws_ssm_parameter.clerk_jwks_url.value
      AUTH_ENABLE_JWT        = "true"
      AUTH_ALLOW_API_KEY     = "true"
      ENVIRONMENT            = var.environment
    }
  }

  depends_on = [
    aws_cloudwatch_log_group.backend,
    aws_iam_role_policy_attachment.backend_basic,
  ]

  tags = local.common_tags
}

resource "aws_lambda_function" "ai" {
  function_name = "${local.prefix}-ai"
  package_type  = "Image"
  image_uri     = var.ai_lambda_image_uri
  role          = aws_iam_role.ai_lambda.arn
  memory_size   = var.ai_lambda_memory
  timeout       = var.ai_lambda_timeout
  architectures = ["arm64"]

  environment {
    variables = {
      AI_PORT                = "8001"
      BACKEND_URL            = "https://${aws_apigatewayv2_api.main.id}.execute-api.${var.aws_region}.amazonaws.com"
      INTERNAL_SERVICE_TOKEN = aws_ssm_parameter.internal_service_token.value
      GEMINI_API_KEY         = aws_ssm_parameter.gemini_api_key.value
      GEMINI_MODEL           = aws_ssm_parameter.gemini_model.value
      AI_ENVIRONMENT         = var.environment
      AI_LOG_JSON            = "true"
    }
  }

  depends_on = [
    aws_cloudwatch_log_group.ai,
    aws_iam_role_policy_attachment.ai_basic,
  ]

  tags = local.common_tags
}

# =============================================================================
# API Gateway (HTTP API – v2)
# =============================================================================

resource "aws_apigatewayv2_api" "main" {
  name          = "${local.prefix}-api"
  protocol_type = "HTTP"

  cors_configuration {
    allow_origins = ["*"]
    allow_methods = ["GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"]
    allow_headers = ["*"]
    max_age       = 3600
  }

  tags = local.common_tags
}

resource "aws_apigatewayv2_stage" "default" {
  api_id      = aws_apigatewayv2_api.main.id
  name        = "$default"
  auto_deploy = true

  access_log_settings {
    destination_arn = aws_cloudwatch_log_group.api_gw.arn
    format = jsonencode({
      requestId      = "$context.requestId"
      ip             = "$context.identity.sourceIp"
      requestTime    = "$context.requestTime"
      httpMethod     = "$context.httpMethod"
      routeKey       = "$context.routeKey"
      status         = "$context.status"
      protocol       = "$context.protocol"
      responseLength = "$context.responseLength"
      integrationErr = "$context.integrationErrorMessage"
    })
  }

  tags = local.common_tags
}

resource "aws_cloudwatch_log_group" "api_gw" {
  name              = "/aws/apigateway/${local.prefix}-api"
  retention_in_days = 30
  tags              = local.common_tags
}

# -- Backend integration -------------------------------------------------------

resource "aws_apigatewayv2_integration" "backend" {
  api_id                 = aws_apigatewayv2_api.main.id
  integration_type       = "AWS_PROXY"
  integration_uri        = aws_lambda_function.backend.invoke_arn
  payload_format_version = "2.0"
}

resource "aws_apigatewayv2_route" "backend_proxy" {
  api_id    = aws_apigatewayv2_api.main.id
  route_key = "ANY /api/{proxy+}"
  target    = "integrations/${aws_apigatewayv2_integration.backend.id}"
}

resource "aws_lambda_permission" "apigw_backend" {
  statement_id  = "AllowAPIGateway"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.backend.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.main.execution_arn}/*/*"
}

# -- AI integration ------------------------------------------------------------

resource "aws_apigatewayv2_integration" "ai" {
  api_id                 = aws_apigatewayv2_api.main.id
  integration_type       = "AWS_PROXY"
  integration_uri        = aws_lambda_function.ai.invoke_arn
  payload_format_version = "2.0"
}

resource "aws_apigatewayv2_route" "ai_proxy" {
  api_id    = aws_apigatewayv2_api.main.id
  route_key = "ANY /ai/{proxy+}"
  target    = "integrations/${aws_apigatewayv2_integration.ai.id}"
}

resource "aws_lambda_permission" "apigw_ai" {
  statement_id  = "AllowAPIGateway"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.ai.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.main.execution_arn}/*/*"
}

# =============================================================================
# Custom Domain (optional – created only when domain_name is set)
# =============================================================================

resource "aws_apigatewayv2_domain_name" "api" {
  count       = var.domain_name != "" ? 1 : 0
  domain_name = var.domain_name

  domain_name_configuration {
    certificate_arn = var.acm_certificate_arn
    endpoint_type   = "REGIONAL"
    security_policy = "TLS_1_2"
  }

  tags = local.common_tags
}

resource "aws_apigatewayv2_api_mapping" "api" {
  count       = var.domain_name != "" ? 1 : 0
  api_id      = aws_apigatewayv2_api.main.id
  domain_name = aws_apigatewayv2_domain_name.api[0].id
  stage       = aws_apigatewayv2_stage.default.id
}

resource "aws_route53_record" "api" {
  count   = var.domain_name != "" && var.route53_zone_id != "" ? 1 : 0
  zone_id = var.route53_zone_id
  name    = var.domain_name
  type    = "A"

  alias {
    name                   = aws_apigatewayv2_domain_name.api[0].domain_name_configuration[0].target_domain_name
    zone_id                = aws_apigatewayv2_domain_name.api[0].domain_name_configuration[0].hosted_zone_id
    evaluate_target_health = false
  }
}

# =============================================================================
# S3 + CloudFront – Frontend Static Hosting
# =============================================================================

resource "aws_s3_bucket" "frontend" {
  bucket = local.frontend_bucket
  tags   = local.common_tags
}

resource "aws_s3_bucket_public_access_block" "frontend" {
  bucket                  = aws_s3_bucket.frontend.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_versioning" "frontend" {
  bucket = aws_s3_bucket.frontend.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_cloudfront_origin_access_control" "frontend" {
  name                              = "${local.prefix}-frontend-oac"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

resource "aws_cloudfront_distribution" "frontend" {
  enabled             = true
  default_root_object = "index.html"
  comment             = "${local.prefix} frontend"
  price_class         = "PriceClass_100"

  origin {
    domain_name              = aws_s3_bucket.frontend.bucket_regional_domain_name
    origin_id                = "s3-frontend"
    origin_access_control_id = aws_cloudfront_origin_access_control.frontend.id
  }

  default_cache_behavior {
    target_origin_id       = "s3-frontend"
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD", "OPTIONS"]
    cached_methods         = ["GET", "HEAD"]
    compress               = true

    forwarded_values {
      query_string = false
      cookies {
        forward = "none"
      }
    }

    min_ttl     = 0
    default_ttl = 3600
    max_ttl     = 86400
  }

  # SPA fallback: serve index.html for client-side routes.
  custom_error_response {
    error_code            = 403
    response_code         = 200
    response_page_path    = "/index.html"
    error_caching_min_ttl = 10
  }

  custom_error_response {
    error_code            = 404
    response_code         = 200
    response_page_path    = "/index.html"
    error_caching_min_ttl = 10
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    cloudfront_default_certificate = true
  }

  tags = local.common_tags
}

# Grant CloudFront access to the S3 bucket.
data "aws_iam_policy_document" "frontend_bucket_policy" {
  statement {
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
  policy = data.aws_iam_policy_document.frontend_bucket_policy.json
}

# =============================================================================
# CloudWatch Monitoring & Alarms
# =============================================================================

resource "aws_cloudwatch_metric_alarm" "backend_errors" {
  alarm_name          = "${local.prefix}-backend-errors"
  alarm_description   = "Backend Lambda error rate exceeded threshold."
  namespace           = "AWS/Lambda"
  metric_name         = "Errors"
  statistic           = "Sum"
  period              = 300
  evaluation_periods  = 1
  threshold           = var.backend_error_threshold
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"

  dimensions = {
    FunctionName = aws_lambda_function.backend.function_name
  }

  alarm_actions = var.alarm_sns_topic_arn != "" ? [var.alarm_sns_topic_arn] : []
  ok_actions    = var.alarm_sns_topic_arn != "" ? [var.alarm_sns_topic_arn] : []

  tags = local.common_tags
}

resource "aws_cloudwatch_metric_alarm" "ai_errors" {
  alarm_name          = "${local.prefix}-ai-errors"
  alarm_description   = "AI Lambda error rate exceeded threshold."
  namespace           = "AWS/Lambda"
  metric_name         = "Errors"
  statistic           = "Sum"
  period              = 300
  evaluation_periods  = 1
  threshold           = var.ai_error_threshold
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"

  dimensions = {
    FunctionName = aws_lambda_function.ai.function_name
  }

  alarm_actions = var.alarm_sns_topic_arn != "" ? [var.alarm_sns_topic_arn] : []
  ok_actions    = var.alarm_sns_topic_arn != "" ? [var.alarm_sns_topic_arn] : []

  tags = local.common_tags
}

resource "aws_cloudwatch_metric_alarm" "backend_duration" {
  alarm_name          = "${local.prefix}-backend-duration"
  alarm_description   = "Backend Lambda p99 latency exceeded 10 s."
  namespace           = "AWS/Lambda"
  metric_name         = "Duration"
  extended_statistic  = "p99"
  period              = 300
  evaluation_periods  = 2
  threshold           = 10000
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"

  dimensions = {
    FunctionName = aws_lambda_function.backend.function_name
  }

  alarm_actions = var.alarm_sns_topic_arn != "" ? [var.alarm_sns_topic_arn] : []

  tags = local.common_tags
}

resource "aws_cloudwatch_metric_alarm" "ai_duration" {
  alarm_name          = "${local.prefix}-ai-duration"
  alarm_description   = "AI Lambda p99 latency exceeded 30 s."
  namespace           = "AWS/Lambda"
  metric_name         = "Duration"
  extended_statistic  = "p99"
  period              = 300
  evaluation_periods  = 2
  threshold           = 30000
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"

  dimensions = {
    FunctionName = aws_lambda_function.ai.function_name
  }

  alarm_actions = var.alarm_sns_topic_arn != "" ? [var.alarm_sns_topic_arn] : []

  tags = local.common_tags
}

resource "aws_cloudwatch_metric_alarm" "api_gw_5xx" {
  alarm_name          = "${local.prefix}-apigw-5xx"
  alarm_description   = "API Gateway 5xx error rate exceeded threshold."
  namespace           = "AWS/ApiGateway"
  metric_name         = "5xx"
  statistic           = "Sum"
  period              = 300
  evaluation_periods  = 1
  threshold           = 10
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"

  dimensions = {
    ApiId = aws_apigatewayv2_api.main.id
  }

  alarm_actions = var.alarm_sns_topic_arn != "" ? [var.alarm_sns_topic_arn] : []

  tags = local.common_tags
}

# -- CloudWatch Dashboard -----------------------------------------------------

resource "aws_cloudwatch_dashboard" "main" {
  dashboard_name = "${local.prefix}-dashboard"

  dashboard_body = jsonencode({
    widgets = [
      {
        type   = "metric"
        x      = 0
        y      = 0
        width  = 12
        height = 6
        properties = {
          title   = "Backend Lambda Invocations & Errors"
          region  = var.aws_region
          metrics = [
            ["AWS/Lambda", "Invocations", "FunctionName", aws_lambda_function.backend.function_name],
            ["AWS/Lambda", "Errors", "FunctionName", aws_lambda_function.backend.function_name],
          ]
          period = 300
          stat   = "Sum"
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 0
        width  = 12
        height = 6
        properties = {
          title   = "AI Lambda Invocations & Errors"
          region  = var.aws_region
          metrics = [
            ["AWS/Lambda", "Invocations", "FunctionName", aws_lambda_function.ai.function_name],
            ["AWS/Lambda", "Errors", "FunctionName", aws_lambda_function.ai.function_name],
          ]
          period = 300
          stat   = "Sum"
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 6
        width  = 12
        height = 6
        properties = {
          title   = "Backend Lambda Duration (p50 / p99)"
          region  = var.aws_region
          metrics = [
            ["AWS/Lambda", "Duration", "FunctionName", aws_lambda_function.backend.function_name, { stat = "p50" }],
            ["AWS/Lambda", "Duration", "FunctionName", aws_lambda_function.backend.function_name, { stat = "p99" }],
          ]
          period = 300
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 6
        width  = 12
        height = 6
        properties = {
          title   = "AI Lambda Duration (p50 / p99)"
          region  = var.aws_region
          metrics = [
            ["AWS/Lambda", "Duration", "FunctionName", aws_lambda_function.ai.function_name, { stat = "p50" }],
            ["AWS/Lambda", "Duration", "FunctionName", aws_lambda_function.ai.function_name, { stat = "p99" }],
          ]
          period = 300
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 12
        width  = 24
        height = 6
        properties = {
          title   = "API Gateway 4xx / 5xx"
          region  = var.aws_region
          metrics = [
            ["AWS/ApiGateway", "4xx", "ApiId", aws_apigatewayv2_api.main.id],
            ["AWS/ApiGateway", "5xx", "ApiId", aws_apigatewayv2_api.main.id],
          ]
          period = 300
          stat   = "Sum"
        }
      },
    ]
  })
}
