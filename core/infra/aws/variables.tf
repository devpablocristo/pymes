variable "project_name" {
  type    = string
  default = "pymes-control-plane"
}

variable "aws_region" {
  type    = string
  default = "us-east-1"
}

variable "lambda_image_uri" {
  type    = string
  default = ""
}

variable "frontend_bucket_name" {
  type    = string
  default = "pymes-control-plane-frontend"
}
