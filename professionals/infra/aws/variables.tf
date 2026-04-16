# -----------------------------------------------------------------------------
# Variables – Professionals Vertical
# -----------------------------------------------------------------------------

variable "project_name" {
  description = "Base name used for resource naming and tagging."
  type        = string
  default     = "pymes-professionals"
}

variable "environment" {
  description = "Deployment environment (dev, staging, prod)."
  type        = string
  default     = "dev"

  validation {
    condition     = contains(["dev", "staging", "prod"], var.environment)
    error_message = "environment must be one of: dev, staging, prod."
  }
}

variable "aws_region" {
  description = "AWS region for all resources."
  type        = string
  default     = "us-east-1"
}

# -- Lambda: Backend (Go) -----------------------------------------------------

variable "backend_lambda_image_uri" {
  description = "ECR image URI for the Go backend Lambda."
  type        = string
}

variable "backend_lambda_memory" {
  description = "Memory (MB) for the backend Lambda."
  type        = number
  default     = 512
}

variable "backend_lambda_timeout" {
  description = "Timeout (seconds) for the backend Lambda."
  type        = number
  default     = 30
}

# -- Lambda: AI (Python FastAPI) -----------------------------------------------

variable "ai_lambda_image_uri" {
  description = "ECR image URI for the Python AI Lambda."
  type        = string
}

variable "ai_lambda_memory" {
  description = "Memory (MB) for the AI Lambda."
  type        = number
  default     = 1024
}

variable "ai_lambda_timeout" {
  description = "Timeout (seconds) for the AI Lambda."
  type        = number
  default     = 60
}

# -- Frontend ------------------------------------------------------------------

variable "frontend_bucket_name" {
  description = "S3 bucket name for the frontend SPA."
  type        = string
  default     = ""
}

# -- Custom Domain -------------------------------------------------------------

variable "domain_name" {
  description = "Custom domain for the API Gateway (leave empty to skip)."
  type        = string
  default     = ""
}

variable "acm_certificate_arn" {
  description = "ACM certificate ARN for the custom domain (required if domain_name is set)."
  type        = string
  default     = ""
}

variable "route53_zone_id" {
  description = "Route 53 hosted zone ID for DNS records (required if domain_name is set)."
  type        = string
  default     = ""
}

# -- Monitoring ----------------------------------------------------------------

variable "alarm_sns_topic_arn" {
  description = "SNS topic ARN for CloudWatch alarm notifications (leave empty to skip)."
  type        = string
  default     = ""
}

variable "backend_error_threshold" {
  description = "Number of backend Lambda errors in 5 min to trigger alarm."
  type        = number
  default     = 5
}

variable "ai_error_threshold" {
  description = "Number of AI Lambda errors in 5 min to trigger alarm."
  type        = number
  default     = 5
}
