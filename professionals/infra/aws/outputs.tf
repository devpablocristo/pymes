# -----------------------------------------------------------------------------
# Outputs – Professionals Vertical
# -----------------------------------------------------------------------------

# -- API Gateway ---------------------------------------------------------------

output "api_gateway_id" {
  description = "HTTP API Gateway ID."
  value       = aws_apigatewayv2_api.main.id
}

output "api_gateway_url" {
  description = "Default invoke URL for the API Gateway."
  value       = aws_apigatewayv2_stage.default.invoke_url
}

output "api_custom_domain" {
  description = "Custom domain name (empty if not configured)."
  value       = var.domain_name != "" ? aws_apigatewayv2_domain_name.api[0].domain_name : ""
}

# -- Lambda --------------------------------------------------------------------

output "backend_lambda_function_name" {
  description = "Backend Lambda function name."
  value       = aws_lambda_function.backend.function_name
}

output "backend_lambda_arn" {
  description = "Backend Lambda ARN."
  value       = aws_lambda_function.backend.arn
}

output "ai_lambda_function_name" {
  description = "AI Lambda function name."
  value       = aws_lambda_function.ai.function_name
}

output "ai_lambda_arn" {
  description = "AI Lambda ARN."
  value       = aws_lambda_function.ai.arn
}

# -- Frontend ------------------------------------------------------------------

output "frontend_bucket_name" {
  description = "S3 bucket hosting the frontend SPA."
  value       = aws_s3_bucket.frontend.id
}

output "frontend_bucket_arn" {
  description = "S3 bucket ARN."
  value       = aws_s3_bucket.frontend.arn
}

output "cloudfront_distribution_id" {
  description = "CloudFront distribution ID (useful for cache invalidation)."
  value       = aws_cloudfront_distribution.frontend.id
}

output "cloudfront_domain_name" {
  description = "CloudFront domain name for the frontend."
  value       = aws_cloudfront_distribution.frontend.domain_name
}

# -- Monitoring ----------------------------------------------------------------

output "cloudwatch_dashboard_name" {
  description = "CloudWatch dashboard name."
  value       = aws_cloudwatch_dashboard.main.dashboard_name
}

# -- SSM Parameter Paths -------------------------------------------------------

output "ssm_parameter_prefix" {
  description = "SSM parameter path prefix for this vertical."
  value       = "/${local.prefix}"
}
