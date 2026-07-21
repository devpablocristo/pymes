variable "name" {
  type = string
}

output "secret_arns" {
  value = {
    clerk  = "arn:aws:secretsmanager:us-east-1:000000000000:secret:${var.name}/clerk"
    stripe = "arn:aws:secretsmanager:us-east-1:000000000000:secret:${var.name}/stripe"
  }
}
