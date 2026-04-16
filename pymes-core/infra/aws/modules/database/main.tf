variable "name" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "private_subnet_ids" {
  type = list(string)
}

output "rds_endpoint" {
  value = "${var.name}.cluster-placeholder.rds.amazonaws.com"
}
