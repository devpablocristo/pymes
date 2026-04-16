# -----------------------------------------------------------------------------
# Outputs – Beauty Vertical
# -----------------------------------------------------------------------------

output "api_gateway_id" {
  description = "HTTP API Gateway ID."
  value       = aws_apigatewayv2_api.main.id
}

output "api_gateway_url" {
  description = "Default invoke URL for the API Gateway (append paths e.g. /v1/beauty/...)."
  value       = aws_apigatewayv2_stage.default.invoke_url
}

output "api_custom_domain" {
  description = "Custom domain name (empty if not configured)."
  value       = var.domain_name != "" ? aws_apigatewayv2_domain_name.api[0].domain_name : ""
}

output "backend_lambda_function_name" {
  description = "Backend Lambda function name."
  value       = aws_lambda_function.backend.function_name
}

output "backend_lambda_arn" {
  description = "Backend Lambda ARN."
  value       = aws_lambda_function.backend.arn
}

output "ssm_parameter_prefix" {
  description = "SSM parameter path prefix for this vertical."
  value       = "/${local.prefix}"
}

output "cloudwatch_dashboard_name" {
  description = "CloudWatch dashboard name."
  value       = aws_cloudwatch_dashboard.main.dashboard_name
}
