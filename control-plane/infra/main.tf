terraform {
  required_version = ">= 1.6.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

module "networking" {
  source = "./modules/networking"
  name   = var.project_name
}

module "database" {
  source            = "./modules/database"
  name              = var.project_name
  vpc_id            = module.networking.vpc_id
  private_subnet_ids = module.networking.private_subnet_ids
}

module "lambda" {
  source              = "./modules/lambda"
  name                = var.project_name
  api_gateway_name    = "${var.project_name}-http-api"
  lambda_image_uri    = var.lambda_image_uri
}

module "cdn" {
  source              = "./modules/cdn"
  name                = var.project_name
  frontend_bucket_name = var.frontend_bucket_name
}

module "secrets" {
  source = "./modules/secrets"
  name   = var.project_name
}

module "monitoring" {
  source = "./modules/monitoring"
  name   = var.project_name
}

module "ecr" {
  source = "./modules/ecr"
  name   = var.project_name
}
