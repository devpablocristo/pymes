variable "name" {
  type = string
}

variable "frontend_bucket_name" {
  type = string
}

output "domain_name" {
  value = "${var.frontend_bucket_name}.cloudfront.net"
}
