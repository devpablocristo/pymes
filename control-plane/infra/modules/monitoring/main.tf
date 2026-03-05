variable "name" {
  type = string
}

output "dashboard_name" {
  value = "${var.name}-dashboard"
}
