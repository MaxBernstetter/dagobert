# VPC for the data plane. Private subnets only: no internet gateway, no NAT (idle cost stays near zero).
# Lambda and ECS tasks join app_security_group_id to reach the database.

locals {
  azs = ["${var.aws_region}a", "${var.aws_region}b", "${var.aws_region}c"]
}

resource "aws_vpc" "main" {
  cidr_block           = var.vpc_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true
  tags                 = { Name = local.prefix }

  depends_on = [terraform_data.deploy_gate]
}

resource "aws_subnet" "private" {
  count             = length(local.azs)
  vpc_id            = aws_vpc.main.id
  availability_zone = local.azs[count.index]
  cidr_block        = cidrsubnet(var.vpc_cidr, 8, count.index)
  tags              = { Name = "${local.prefix}-private-${local.azs[count.index]}" }
}

resource "aws_security_group" "app" {
  name        = "${local.prefix}-app"
  description = "Workloads (Lambda, ECS) that may reach the database"
  vpc_id      = aws_vpc.main.id
}

resource "aws_security_group" "database" {
  name        = "${local.prefix}-database"
  description = "Aurora: PostgreSQL from workloads only"
  vpc_id      = aws_vpc.main.id

  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.app.id]
  }
}

resource "aws_db_subnet_group" "main" {
  name       = local.prefix
  subnet_ids = aws_subnet.private[*].id
}
