variable "name" {
  type = string
}

output "vpc_id" {
  value = "vpc-placeholder"
}

output "private_subnet_ids" {
  value = ["subnet-private-a", "subnet-private-b"]
}
