output "vpc_id" {
  value = module.networking.vpc_id
}

output "api_gateway_url" {
  value = module.lambda.api_gateway_url
}

output "frontend_domain_name" {
  value = module.cdn.domain_name
}
