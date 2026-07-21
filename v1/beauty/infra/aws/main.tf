# =============================================================================
# Terraform – Beauty Vertical (AWS)
# Backend Go (Lambda + HTTP API). Estado independiente del control-plane.
# El frontend unificado se despliega aparte (pymes-core / pipeline global).
# =============================================================================

terraform {
  required_version = ">= 1.6.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  # backend "s3" { ... }
}

provider "aws" {
  region = var.aws_region
}

locals {
  prefix = "${var.project_name}-${var.environment}"

  common_tags = {
    Project     = var.project_name
    Vertical    = "beauty"
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

resource "aws_ssm_parameter" "pymes_core_url" {
  name  = "/${local.prefix}/PYMES_CORE_URL"
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
  name  = "/${local.prefix}/JWT_ISSUER"
  type  = "String"
  value = "CHANGE_ME"

  tags = local.common_tags

  lifecycle {
    ignore_changes = [value]
  }
}

resource "aws_ssm_parameter" "clerk_jwks_url" {
  name  = "/${local.prefix}/JWKS_URL"
  type  = "String"
  value = "CHANGE_ME"

  tags = local.common_tags

  lifecycle {
    ignore_changes = [value]
  }
}

resource "aws_ssm_parameter" "frontend_url" {
  name  = "/${local.prefix}/FRONTEND_URL"
  type  = "String"
  value = "CHANGE_ME"

  tags = local.common_tags

  lifecycle {
    ignore_changes = [value]
  }
}

# =============================================================================
# IAM – Lambda
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

# =============================================================================
# CloudWatch Log Group
# =============================================================================

resource "aws_cloudwatch_log_group" "backend" {
  name              = "/aws/lambda/${local.prefix}-backend"
  retention_in_days = 30
  tags              = local.common_tags
}

# =============================================================================
# Lambda
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
      PORT                   = "8083"
      DATABASE_URL           = aws_ssm_parameter.database_url.value
      PYMES_CORE_URL         = aws_ssm_parameter.pymes_core_url.value
      INTERNAL_SERVICE_TOKEN = aws_ssm_parameter.internal_service_token.value
      JWT_ISSUER             = aws_ssm_parameter.clerk_issuer.value
      JWKS_URL               = aws_ssm_parameter.clerk_jwks_url.value
      AUTH_ENABLE_JWT        = "true"
      AUTH_ALLOW_API_KEY     = "true"
      FRONTEND_URL           = aws_ssm_parameter.frontend_url.value
      ENVIRONMENT            = var.environment
    }
  }

  depends_on = [
    aws_cloudwatch_log_group.backend,
    aws_iam_role_policy_attachment.backend_basic,
  ]

  tags = local.common_tags
}

# =============================================================================
# API Gateway HTTP API (v2)
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

  tags = local.common_tags
}

resource "aws_apigatewayv2_integration" "backend" {
  api_id                 = aws_apigatewayv2_api.main.id
  integration_type       = "AWS_PROXY"
  integration_uri        = aws_lambda_function.backend.invoke_arn
  payload_format_version = "2.0"
}

resource "aws_apigatewayv2_route" "backend_proxy" {
  api_id    = aws_apigatewayv2_api.main.id
  route_key = "ANY /{proxy+}"
  target    = "integrations/${aws_apigatewayv2_integration.backend.id}"
}

resource "aws_lambda_permission" "apigw_backend" {
  statement_id  = "AllowAPIGateway"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.backend.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.main.execution_arn}/*/*"
}

# =============================================================================
# Custom Domain (optional)
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
# CloudWatch Alarms
# =============================================================================

resource "aws_cloudwatch_metric_alarm" "backend_errors" {
  alarm_name          = "${local.prefix}-backend-errors"
  alarm_description   = "Beauty backend Lambda error rate exceeded threshold."
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

resource "aws_cloudwatch_dashboard" "main" {
  dashboard_name = "${local.prefix}-dashboard"

  dashboard_body = jsonencode({
    widgets = [
      {
        type   = "metric"
        x      = 0
        y      = 0
        width  = 24
        height = 6
        properties = {
          title  = "Beauty Backend Lambda – Invocations & Errors"
          region = var.aws_region
          metrics = [
            ["AWS/Lambda", "Invocations", "FunctionName", aws_lambda_function.backend.function_name],
            [".", "Errors", ".", "."],
          ]
          period = 300
          stat   = "Sum"
        }
      },
    ]
  })
}
