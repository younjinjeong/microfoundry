# MicroFoundry Topology — Amazon RDS PostgreSQL

terraform {
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

locals {
  db_name = replace(var.instance_name, "-", "_")

  instance_class = (
    var.plan_memory_mb <= 1024 ? "db.t3.micro" :
    var.plan_memory_mb <= 4096 ? "db.t3.medium" :
    "db.r6g.large"
  )
}

resource "aws_db_subnet_group" "this" {
  name       = "mf-${var.instance_name}"
  subnet_ids = var.subnet_ids

  tags = {
    Name                           = "mf-${var.instance_name}"
    "app.kubernetes.io/managed-by" = "microfoundry"
  }
}

resource "aws_security_group" "this" {
  name        = "mf-rds-${var.instance_name}"
  description = "MicroFoundry RDS PostgreSQL - ${var.instance_name}"
  vpc_id      = var.vpc_id

  ingress {
    description = "PostgreSQL"
    from_port   = 5432
    to_port     = 5432
    protocol    = "tcp"
    cidr_blocks = ["10.0.0.0/8"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name                           = "mf-rds-${var.instance_name}"
    "app.kubernetes.io/managed-by" = "microfoundry"
  }
}

resource "aws_db_instance" "this" {
  identifier     = "mf-${var.instance_name}"
  engine         = "postgres"
  engine_version = "16.4"
  instance_class = local.instance_class

  allocated_storage     = var.plan_storage_gb
  max_allocated_storage = var.plan_storage_gb * 2
  storage_type          = "gp3"

  db_name  = local.db_name
  username = "mfadmin"
  password = var.password

  db_subnet_group_name   = aws_db_subnet_group.this.name
  vpc_security_group_ids = [aws_security_group.this.id]

  multi_az            = var.plan_memory_mb >= 16384
  skip_final_snapshot = true
  deletion_protection = false

  backup_retention_period = 7
  copy_tags_to_snapshot   = true

  tags = {
    "app.kubernetes.io/managed-by" = "microfoundry"
    "microfoundry.io/instance"     = var.instance_name
    "microfoundry.io/namespace"    = var.namespace
  }
}
