variable "name" {
  type = string
}

variable "api_gateway_name" {
  type = string
}

variable "lambda_image_uri" {
  type = string
}

output "api_gateway_url" {
  value = "https://api-placeholder.execute-api.us-east-1.amazonaws.com"
}
